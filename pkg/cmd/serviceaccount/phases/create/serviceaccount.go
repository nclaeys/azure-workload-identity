package phases

import (
	"context"
	"time"

	"github.com/Azure/azure-workload-identity/pkg/cmd/serviceaccount/options"
	"github.com/Azure/azure-workload-identity/pkg/cmd/serviceaccount/phases/workflow"
	"github.com/Azure/azure-workload-identity/pkg/kuberneteshelper"
	"github.com/Azure/azure-workload-identity/pkg/webhook"
	"k8s.io/client-go/kubernetes"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	serviceAccountPhaseName = "service-account"
)

type serviceAccountPhase struct {
	kubeClient kubernetes.Interface
}

// NewServiceAccountPhase creates a new phase to create a Kubernetes service account
func NewServiceAccountPhase() workflow.Phase {
	p := &serviceAccountPhase{}
	return workflow.Phase{
		Name:        serviceAccountPhaseName,
		Aliases:     []string{"sa"},
		Description: "Create Kubernetes service account in the current KUBECONFIG context and add azure-workload-identity labels and annotations to it",
		PreRun:      p.prerun,
		Run:         p.run,
		Flags: []string{
			options.ServiceAccountNamespace.Flag,
			options.ServiceAccountName.Flag,
			options.ServiceAccountTokenExpiration.Flag,
			options.AADApplicationName.Flag,
			options.AADApplicationClientID.Flag,
		},
	}
}

func (p *serviceAccountPhase) prerun(data workflow.RunData) error {
	createData, ok := data.(CreateData)
	if !ok {
		return errors.Errorf("invalid data type %T", data)
	}

	if createData.ServiceAccountNamespace() == "" {
		return options.FlagIsRequiredError(options.ServiceAccountNamespace.Flag)
	}
	if createData.ServiceAccountName() == "" {
		return options.FlagIsRequiredError(options.ServiceAccountName.Flag)
	}

	minTokenExpirationDuration := time.Duration(webhook.MinServiceAccountTokenExpiration) * time.Second
	maxTokenExpirationDuration := time.Duration(webhook.MaxServiceAccountTokenExpiration) * time.Second
	if createData.ServiceAccountTokenExpiration() < minTokenExpirationDuration {
		return errors.Errorf("--service-account-token-expiration must be greater than or equal to %s", minTokenExpirationDuration.String())
	}
	if createData.ServiceAccountTokenExpiration() > maxTokenExpirationDuration {
		return errors.Errorf("--service-account-token-expiration must be less than or equal to %s", maxTokenExpirationDuration.String())
	}

	var err error
	if p.kubeClient, err = createData.KubeClient(); err != nil {
		return errors.Wrap(err, "failed to get kubernetes client")
	}

	return nil
}

func (p *serviceAccountPhase) run(ctx context.Context, data workflow.RunData) error {
	createData := data.(CreateData)

	// TODO(aramase) make the update behavior configurable. If the service account already exists, fail if --overwrite is not specified
	err := kuberneteshelper.CreateOrUpdateServiceAccount(
		ctx,
		p.kubeClient,
		createData.ServiceAccountNamespace(),
		createData.ServiceAccountName(),
		createData.AADApplicationClientID(),
		createData.AzureTenantID(),
		createData.ServiceAccountTokenExpiration(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create kubernetes service account")
	}

	log.WithFields(log.Fields{
		"namespace": createData.ServiceAccountNamespace(),
		"name":      createData.ServiceAccountName(),
	}).Infof("[%s] created kubernetes service account", serviceAccountPhaseName)

	return nil
}
