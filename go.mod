module github.com/aokumasan/cert-manager-nifcloud-webhook

go 1.16

require (
	github.com/go-acme/lego/v4 v4.3.2-0.20210526163128-81bf297aa649
	github.com/jetstack/cert-manager v1.3.1
	github.com/nifcloud/nifcloud-sdk-go v1.12.0
	github.com/pkg/errors v0.9.1
	k8s.io/apiextensions-apiserver v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
)
