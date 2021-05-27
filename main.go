package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aokumasan/cert-manager-nifcloud-webhook/internal"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/platform/wait"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
	cmmetav1 "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	"github.com/pkg/errors"
)

var GroupName = os.Getenv("GROUP_NAME")

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(GroupName,
		&nifcloudDNSProviderSolver{},
	)
}

// nifcloudDNSProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface.
type nifcloudDNSProviderSolver struct {
	// If a Kubernetes 'clientset' is needed, you must:
	// 1. uncomment the additional `client` field in this structure below
	// 2. uncomment the "k8s.io/client-go/kubernetes" import at the top of the file
	// 3. uncomment the relevant code in the Initialize method below
	// 4. ensure your webhook's service account has the required RBAC role
	//    assigned to it for interacting with the Kubernetes APIs you need.
	client         *kubernetes.Clientset
	nifcloudClient *internal.Client
}

// nifcloudDNSProviderConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
// This information is provided by cert-manager, and may be a reference to
// additional configuration that's needed to solve the challenge for this
// particular certificate or issuer.
// This typically includes references to Secret resources containing DNS
// provider credentials, in cases where a 'multi-tenant' DNS solver is being
// created.
// If you do *not* require per-issuer or per-certificate configuration to be
// provided to your webhook, you can skip decoding altogether in favour of
// using CLI flags or similar to provide configuration.
// You should not include sensitive information here. If credentials need to
// be used by your provider here, you should reference a Kubernetes Secret
// resource and fetch these credentials using a Kubernetes clientset.
type nifcloudDNSProviderConfig struct {
	// Change the two fields below according to the format of the configuration
	// to be decoded.
	// These fields will be set by users in the
	// `issuer.spec.acme.dns01.providers.webhook.config` field.

	AccessKeyIDSecretRef     cmmetav1.SecretKeySelector `json:"accessKeyIdSecretRef"`
	SecretAccessKeySecretRef cmmetav1.SecretKeySelector `json:"secretAccessKeySecretRef"`
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *nifcloudDNSProviderSolver) Name() string {
	return "nifcloud-solver"
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *nifcloudDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	if c.nifcloudClient == nil {
		c.prepareNifcloudClient(ch)
	}

	return c.changeRecord("CREATE", ch.ResolvedFQDN, ch.Key, dns01.DefaultTTL)
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *nifcloudDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	if c.nifcloudClient == nil {
		c.prepareNifcloudClient(ch)
	}

	return c.changeRecord("DELETE", ch.ResolvedFQDN, ch.Key, dns01.DefaultTTL)
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *nifcloudDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.client = cl

	return nil
}

func (c *nifcloudDNSProviderSolver) prepareNifcloudClient(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	accessKeyID, err := c.loadSecretData(cfg.AccessKeyIDSecretRef, ch.ResourceNamespace)
	secretAccessKey, err := c.loadSecretData(cfg.SecretAccessKeySecretRef, ch.ResourceNamespace)
	if err != nil {
		return err
	}

	client, err := internal.NewClient(string(accessKeyID), string(secretAccessKey))
	if err != nil {
		return err
	}

	c.nifcloudClient = client

	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (nifcloudDNSProviderConfig, error) {
	cfg := nifcloudDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}

func (c *nifcloudDNSProviderSolver) loadSecretData(selector cmmetav1.SecretKeySelector, ns string) ([]byte, error) {
	secret, err := c.client.CoreV1().Secrets(ns).Get(context.Background(), selector.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load secret %q", ns+"/"+selector.Name)
	}

	if data, ok := secret.Data[selector.Key]; ok {
		return data, nil
	}

	return nil, errors.Errorf("no key %q in secret %q", selector.Key, ns+"/"+selector.Name)
}

func (c *nifcloudDNSProviderSolver) changeRecord(action, fqdn, value string, ttl int) error {
	name := dns01.UnFqdn(fqdn)

	reqParams := internal.ChangeResourceRecordSetsRequest{
		XMLNs: internal.XMLNs,
		ChangeBatch: internal.ChangeBatch{
			Comment: "Managed by cert-manager-nifcloud-webhook",
			Changes: internal.Changes{
				Change: []internal.Change{
					{
						Action: action,
						ResourceRecordSet: internal.ResourceRecordSet{
							Name: name,
							Type: "TXT",
							TTL:  ttl,
							ResourceRecords: internal.ResourceRecords{
								ResourceRecord: []internal.ResourceRecord{
									{
										Value: value,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	authZone, err := dns01.FindZoneByFqdn(fqdn)
	if err != nil {
		return fmt.Errorf("failed to find zone: %w", err)
	}

	resp, err := c.nifcloudClient.ChangeResourceRecordSets(dns01.UnFqdn(authZone), reqParams)
	if err != nil {
		return fmt.Errorf("failed to change record set: %w", err)
	}

	statusID := resp.ChangeInfo.ID

	return wait.For("nifcloud", 120*time.Second, 4*time.Second, func() (bool, error) {
		resp, err := c.nifcloudClient.GetChange(statusID)
		if err != nil {
			return false, fmt.Errorf("failed to query change status: %w", err)
		}
		return resp.ChangeInfo.Status == "INSYNC", nil
	})
}
