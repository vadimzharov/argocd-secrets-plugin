## Generator for ArgoCD to use values from K8S secrets in ApplicationSets

This is the simple [generator](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Generators-Plugin/) for ArgoCD to read values from K8S secrets and use these values in ApplicationSet/Application manifests. The idea is to implement in ArgoCD the same feature as [FluxCD valuesFrom](https://fluxcd.io/flux/components/helm/helmreleases/#values-references).

### Installation

To understand the installation process, read the [official ArgoCD ApplicationSets plugin guide](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Generators-Plugin/#add-a-configmap-to-configure-the-access-of-the-plugin)

Manifests to deploy and configure the plugin are in [install/manifests](install/manifests/) folder:
* `configmap.yaml` - K8S ConfigMap with plugin configuration. The ConfigMap is not to configure the plugin - name of this ConfigMap needs to be referenced in an ApplicationSet. ConfigMap contains plugin URL, request timeout value and reference to a K8S secret with authentication token (ArgoCD uses this token to authenticate all requiests to the plugin).
* `secret.yaml` - K8S Secret with token. ArgoCD use the token to connect to the plugin API. **Update value in this secret before applying the manifest.** This value is referenced in the plugin deployment (`deployment.yaml`, `ARGOCD_PLUGIN_TOKEN` environment variable) and in the ConfigMap with plugin configuration.
* `deployment.yaml` - manifets to create Service Account, Deployment and Service to run the plugin. The Service account is referenced in the plugin deployment, and this service account **must** have permissions to *get* (read content of) K8S secrets - this is configured by `rbac.yaml` manifest. 
* `rbac.yaml` - manifests to configure Role and RoleBinding for the plugin Service Account so it can *get* secrets in ArgoCD namespace (`argocd`). This is the default configuration, Role/RoleBinding can be modified so the Service account *get* secrets in any other namespace (see **Usage** part how to reference namespace other than `argocd` in ApplicationSet).


To install the plugin:

1. Clone the repository
2. Review manifests in `install/manifests` directory
3. Update secret `argo-secrets-sync` in "secret.yaml" file, key `plugin.argo-secrets-sync.token` with some random string
4. Deploy all manifests by executing `kubectl apply -f install/manifests/`

This will install the plugin in the default configuration - in `argocd` namespace with permissions for the plugin to read all secrets in `argocd` namespace.

### Usage

As an input parameter (`secretName`) plugin requires name of the secret in `argocd` namespace. Then, during ApplicationSet processing, plugin reads **all** key/value pairs from this secret and passes it to ArgoCD. Users need to reference secret key as `{{ <keyname> }}` in the ApplicationSet manifest - and then ArgoCD replaces it with key value during Application template processing.

For example, if the generator is set to read from secret `my-super-secret` in the ApplicationSet manifest - then key named `my-variable` can be used in an Application template:
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
                  loadBalancerIP: {{ my-variable }}  # If secret has key my-variable - then this will be replaced with value
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

Assume we need to deploy Nginx application (based on Nginx Helm chart)  with ArgoCD where we don't want to store Nginx LoadBalancer IP inside Git repo, but rather want for ArgoCD to set this parameter from K8S secret.

Create a secret with the key/value:
```
apiVersion: v1
kind: Secret
metadata:
  name: secret-variables
  namespace: argocd
stringData:
  lbip: 192.168.1.1
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
          # Specify the the secret with keys/values we need to use inside Application template
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
                  # The plugin reads secret "cluster-secrets" and if contains key "lbip" replaces lbip variable with value from the secret
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

Create a secret vith LoadBalancer IP for `clusterone` so the name of the secret contains name of the cluster in ArgoCD (`clusterone`):

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

