package main

import (
	pluginkube "argocd-secrets-plugin/internal/pluginkube"
	pluginrouter "argocd-secrets-plugin/internal/pluginrouter"
)

func main() {

	clientset := pluginkube.KubeClientSet()

	pluginrouter.RouterSet(clientset)

}
