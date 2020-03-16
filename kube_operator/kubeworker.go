package kube_operator

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/events"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/worker"
)

type KubeWorker struct {
	worker.BaseWorker
	db *bolt.DB
}

func NewKubeWorker(name string, config *config.HorizonConfig, db *bolt.DB) *KubeWorker {
	worker := &KubeWorker{
		BaseWorker: worker.NewBaseWorker(name, config, nil),
		db:         db,
	}
	glog.Info(kwlog(fmt.Sprintf("Starting Kubernetes Worker")))
	worker.Start(worker, 0)
	return worker
}

func (w *KubeWorker) Messages() chan events.Message {
	return w.BaseWorker.Manager.Messages
}

func (w *KubeWorker) NewEvent(incoming events.Message) {
	switch incoming.(type) {
	case *events.AgreementReachedMessage:
		msg, _ := incoming.(*events.AgreementReachedMessage)

		fCmd := NewInstallCommand(msg.LaunchContext())
		w.Commands <- fCmd
	case *events.GovernanceWorkloadCancelationMessage:
		msg, _ := incoming.(*events.GovernanceWorkloadCancelationMessage)

		switch msg.Event().Id {
		case events.AGREEMENT_ENDED:
			cmd := NewUnInstallCommand(msg.AgreementProtocol, msg.AgreementId, msg.Deployment)
			w.Commands <- cmd
		}

	case *events.GovernanceMaintenanceMessage:
		msg, _ := incoming.(*events.GovernanceMaintenanceMessage)

		switch msg.Event().Id {
		case events.CONTAINER_MAINTAIN:
			cmd := NewMaintenanceCommand(msg.AgreementProtocol, msg.AgreementId, msg.Deployment)
			w.Commands <- cmd
		}

	case *events.NodeShutdownCompleteMessage:
		msg, _ := incoming.(*events.NodeShutdownCompleteMessage)
		switch msg.Event().Id {
		case events.UNCONFIGURE_COMPLETE:
			w.Commands <- worker.NewTerminateCommand("shutdown")
		}

	default: //nothing

	}
	return
}

func (w *KubeWorker) CommandHandler(command worker.Command) bool {
	switch command.(type) {
	case *InstallCommand:
		cmd := command.(*InstallCommand)
		if lc := w.getLaunchContext(cmd.LaunchContext); lc == nil {
			glog.Errorf(kwlog(fmt.Sprintf("incoming event was not a known launch context %T", cmd.LaunchContext)))
		} else {
			glog.V(5).Infof(kwlog(fmt.Sprintf("LaunchContext(%T): %v", lc, lc)))

			// Check the deployment to check if it is a kube deployment
			deploymentConfig := lc.ContainerConfig().Deployment
			if kd, err := persistence.GetKubeDeployment(deploymentConfig); err != nil {
				glog.Warningf(kwlog(fmt.Sprintf("ignoring non-Kube deployment: %v", err)))
				return true
			} else if _, err := persistence.AgreementDeploymentStarted(w.db, lc.AgreementId, lc.AgreementProtocol, kd); err != nil {
				glog.Errorf(kwlog(fmt.Sprintf("received error updating database deployment state, %v", err)))
				w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, lc.AgreementProtocol, lc.AgreementId, kd)
				return true
			} else if err := w.processKubeOperator(lc, kd); err != nil {
				glog.Errorf(kwlog(fmt.Sprintf("failed to process kube package after agreement negotiation: %v", err)))
				w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, lc.AgreementProtocol, lc.AgreementId, kd)
				return true
			} else {
				w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_BEGUN, lc.AgreementProtocol, lc.AgreementId, kd)
			}
		}
	case *UnInstallCommand:
		cmd := command.(*UnInstallCommand)
		glog.V(5).Infof(kwlog(fmt.Sprintf("uninstalling %v", cmd.Deployment)))

		kdc, ok := cmd.Deployment.(*persistence.KubeDeploymentConfig)
		if !ok {
			glog.Warningf(kwlog(fmt.Sprintf("ignoring non-kube deployment %v", cmd.Deployment)))
			return true
		} else if err := w.uninstallKubeOperator(kdc); err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("failed to uninstall kube operator %v", cmd.Deployment)))
		}

		w.Messages() <- events.NewWorkloadMessage(events.WORKLOAD_DESTROYED, cmd.AgreementProtocol, cmd.CurrentAgreementId, kdc)
	case *MaintenanceCommand:
		cmd := command.(*MaintenanceCommand)
		glog.V(5).Infof(kwlog(fmt.Sprintf("recieved maintenence command %v", cmd)))

		kdc, ok := cmd.Deployment.(*persistence.KubeDeploymentConfig)
		if !ok {
			glog.Warningf(kwlog("ignoring non-Kube maintenence command: %v, cmd"))
		} else if err := w.operatorStatus(kdc, "DEPLOYED"); err != nil {
			glog.Errorf(kwlog(fmt.Sprintf("%v", err)))
			w.Messages() <- events.NewWorkloadMessage(events.EXECUTION_FAILED, cmd.AgreementProtocol, cmd.AgreementId, kdc)
		}
	default:
		return true
	}
	return true
}

func (w *KubeWorker) getLaunchContext(launchContext interface{}) *events.AgreementLaunchContext {
	switch launchContext.(type) {
	case *events.AgreementLaunchContext:
		lc := launchContext.(*events.AgreementLaunchContext)
		return lc
	}
	return nil
}

func (w *KubeWorker) processKubeOperator(launchContext *events.AgreementLaunchContext, kd *persistence.KubeDeploymentConfig) error {
	glog.V(5).Infof(kwlog(fmt.Sprintf("begin install of Kube Deployment %v", kd.Operator_Image)))
	// Start install here
	return nil
}

func (w *KubeWorker) uninstallKubeOperator(kd *persistence.KubeDeploymentConfig) error {
	glog.V(5).Infof(kwlog(fmt.Sprintf("begin uninstall of Kube Deployment %v", kd.Operator_Image)))
	// Start uninstall here
	return nil
}

func (w *KubeWorker) operatorStatus(kd *persistence.KubeDeploymentConfig, state string) error {
	glog.V(5).Infof(kwlog(fmt.Sprintf("begin listing operator status %v", kd)))
	// Find status here
	return nil
}

var kwlog = func(v interface{}) string {
	return fmt.Sprintf("Kubernetes Worker: %v", v)
}