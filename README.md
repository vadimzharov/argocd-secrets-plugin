## Generator for ArgoCD to use values from K8S secrets in ApplicationSets

This is the simple [generator](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Generators-Plugin/) for ArgoCD to read values from K8S secrets and use these values in ApplicationSet/Application manifests. The idea is to implement in ArgoCD the same feature as [FluxCD valuesFrom](https://fluxcd.io/flux/components/helm/helmreleases/#values-references).

### Installation

To understand the installation process, read the [official ArgoCD ApplicationSets plugin guide](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Generators-Plugin/#add-a-configmap-to-configure-the-access-of-the-plugin)

Manifests to deploy and configure the plugin are in [install/manifests](install/manifests/) folder:
* `configmap.yaml` - ConfigMap with plugin configuration. Name of this ConfigMap needs to be referenced in ApplicationSet
* `secret.yaml` - Secret with token ArgoCD use to connect to the plugin API. Update value in this secret before applying the manifest. This value is referenced in plugin deploymen (`deployment.yaml`, `ARGOCD_PLUGIN_TOKEN` environment variable) and in ConfigMap with plugin configuration
* `deployment.yaml` - manifets to create Service Account, Deployment and Service. The Service account is referenced in plugin deployment, and this service account **must** have permissions to *get* (read content of) K8S secrets - this is configured by `rbac.yaml` manifest. 
* `rbac.yaml` - manifests to configure Role and RoleBinding for plugin service account so it can *get* secrets in ArgoCD namespace (`argocd`). This is the default configuration, Role/RoleBinding can be modified so the Service account *get* secrets in any other namespace (see **Usage** part how to reference namespace other than `argocd` in ApplicationSet)


To install the plugin:

1. Clone the repository
2. Review manifests in `install/manifests` directory
3. Update secret `argo-secrets-sync` in "secret.yaml" file, key `plugin.argo-secrets-sync.token` with some random string
4. Deploy all manifests by executing `kubectl apply -f install/manifests/`

This will install the plugin in default configuration - in `argocd` namespace with permissions for the plugin to read all secrets in `argocd` namespace.

### Usage

As an input parameter (`secretName`) plugin requires name of the secret in `argocd` namespace. Then, during ApplicationSet processing, plugin reads all key/value pairs from this secret and passes it to ArgoCD. Users need to reference secret key as `{{ <keyname> }}` in the ApplicationSet manifest - and then ArgoCD replace it with key value during Application template processing.

For example, if the generator is set to read from secret `my-super-secret` in the ApplicationSet manifest - then key named `my-variable` can be used in Application template:
```
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: myplugin
.......  
  generators:
    - plugin:
        # Specify the configMap where the plugin configuration is located.
        configMapRef:
          name: my-plugin
        # You can pass arbitrary parameters to the plugin. `input.parameters` is a map, but values may be any type.
        # These parameters will also be available on the generator's output under the `generator.input.parameters` key.
        input:
          parameters:
            secretName: "my-super-secret"  # Plugin will read all keys from secret my-super-secret in argocd namespace
.........
  template:
    metadata:
      name: 'plugin-nginx'
    spec:
      destination:
        namespace: ingress-nginx-appset
        server: '{{server}}'
      project: default
      sources:
        - chart: ingress-nginx
          helm:
            releaseName: nginx
            values: |
              controller:
                service:
                  enabled: true
                  loadBalancerIP: {{ my-variable }}  # If secret has key my-super-secret - then this will be replaced with value
                ingressClassResource:
                  default: false
          repoURL: https://kubernetes.github.io/ingress-nginx
          targetRevision: 4.11.2

```

Then, if K8S secret `my-super-secret` have key `my-variable` set to `192.168.1.1` then application template will have:

```
      sources:
        - chart: ingress-nginx
          helm:
            releaseName: nginx
            values: |
              controller:
                service:
                  enabled: true
                  loadBalancerIP: 192.168.1.1
                ingressClassResource:
                  default: false
```

### Simple use-case example - use values from secret in ApplicationSet

Assume we need to deploy Nginx application (based on ) with ArgoCD where we don't want to store Nginx LoadBalancer IP inside Git repo, but rather want for ArgoCD to set this parameter from K8S secret.

Create secret with the key/value:
```
apiVersion: v1
kind: Secret
metadata:
  name: secret-variables
  namespace: argocd
stringData:
  lbip: 192.168.1.1
type: kubernetes.io/basic-auth
```


Example of the ApplicationSet manifest:
```
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: plugin-test
  namespace: argocd
spec:
  generators:
    - plugin:
        # Specify the configMap where the plugin configuration is located.
        configMapRef:
          name: my-plugin
        input:
          parameters:
            secretName: "cluster-secrets"
  template:
    metadata:
      name: 'plugin-nginx'
    spec:
      destination:
        namespace: ingress-nginx-appset
        server: '{{server}}'
      project: default
      sources:
        - chart: ingress-nginx
          helm:
            releaseName: nginx
            values: |
              controller:
                service:
                  enabled: true
                  loadBalancerIP: {{ lbip }}
                ingressClassResource:
                  default: false
          repoURL: https://kubernetes.github.io/ingress-nginx
          targetRevision: 4.11.2
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
          - CreateNamespace=true

```

### More advanced use-case example - combine plugin with clusters generator

This is the use-case I've created this plugin for - to be able to manage multiple clusters from one ArgoCD instance and use the same ApplicationSet for any number of clusters in cases where each cluster needs it's own values.

For example, assuming ArgoCD is managing two clusters named `clusterone` and `clustertwo`, and the requirement is to deploy nginx ingress controller via Helm chart, where `clusterone` must use LoadBalancer IP `192.168.1.1` and `clustertwo` must use LoadBalancer IP `192.168.100.100`.

Create secret vith LoadBalancer IP for `clusterone`:

```
apiVersion: v1
kind: Secret
metadata:
  name: clusterone-secrets
  namespace: argocd
stringData:
  loadbalancerip: 192.168.1.1
```

And for `clustertwo`:

```
apiVersion: v1
kind: Secret
metadata:
  name: clustertwo-secrets
  namespace: argocd
stringData:
  loadbalancerip: 192.168.100.100
```

Create ApplicationSet with matrix generator, combining cluster and plugin generators:

```
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: plugin-matrix-test
  namespace: argocd
spec:
  generators:
    - matrix:
        generators:
          - clusters:
            selector:
              matchLabels:
                nginx: "installed"
          - plugin:
              # Specify the configMap where the plugin configuration is located.
              configMapRef:
                name: argocd-secrets-plugin
              input:
                parameters:
                  secretName: "{{ name }}-secrets"
  template:
    metadata:
      name: 'plugin-nginx'
    spec:
      destination:
        namespace: ingress-nginx-appset
        server: '{{server}}'
      project: default
      sources:
        - chart: ingress-nginx
          helm:
            releaseName: nginx
            values: |
              controller:
                service:
                  enabled: true
                  loadBalancerIP: {{ loadbalancerip }}
                ingressClassResource:
                  default: false
          repoURL: https://kubernetes.github.io/ingress-nginx
          targetRevision: 4.11.2
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
          - CreateNamespace=true

```

In this case, when application is being generated and deployed to `clusterone` cluster then it will use value `192.168.1.1` from secret `clusterone-secrets` to configure LoadBalancer IP, and value `192.168.100.100` from secret `clustertwo` if it is being generated and deployed to `clustertwo`.

