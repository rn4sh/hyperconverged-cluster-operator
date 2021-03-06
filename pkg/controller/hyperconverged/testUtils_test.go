package hyperconverged

import (
	"context"
	"fmt"
	sspopv1 "github.com/MarSik/kubevirt-ssp-operator/pkg/apis"
	sspv1 "github.com/MarSik/kubevirt-ssp-operator/pkg/apis/kubevirt/v1"
	networkaddons "github.com/kubevirt/cluster-network-addons-operator/pkg/apis"
	networkaddonsv1alpha1 "github.com/kubevirt/cluster-network-addons-operator/pkg/apis/networkaddonsoperator/v1alpha1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis"
	hcov1alpha1 "github.com/kubevirt/hyperconverged-cluster-operator/pkg/apis/hco/v1alpha1"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/version"
	vmimportv1 "github.com/kubevirt/vm-import-operator/pkg/apis/v2v/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type basicExpected struct {
	hco             *hcov1alpha1.HyperConverged
	pc              *schedulingv1.PriorityClass
	kvConfig        *corev1.ConfigMap
	kvStorageConfig *corev1.ConfigMap
	kv              *kubevirtv1.KubeVirt
	cdi             *cdiv1alpha1.CDI
	cna             *networkaddonsv1alpha1.NetworkAddonsConfig
	kvCtb           *sspv1.KubevirtCommonTemplatesBundle
	kvNlb           *sspv1.KubevirtNodeLabellerBundle
	kvTv            *sspv1.KubevirtTemplateValidator
	vmi             *vmimportv1.VMImportConfig
	kvMtAg          *sspv1.KubevirtMetricsAggregation
	imsConfig       *corev1.ConfigMap
}

func (be basicExpected) toArray() []runtime.Object {
	return []runtime.Object{
		be.hco,
		be.pc,
		be.kvConfig,
		be.kvStorageConfig,
		be.kv,
		be.cdi,
		be.cna,
		be.kvCtb,
		be.kvNlb,
		be.kvTv,
		be.vmi,
		be.kvMtAg,
		be.imsConfig,
	}
}

func (be basicExpected) initClient() *hcoTestClient {
	return initClient(be.toArray())
}

// Mock request to simulate Reconcile() being called on an event for a watched resource
var (
	request = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
)

func newHco() *hcov1alpha1.HyperConverged {
	return &hcov1alpha1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: hcov1alpha1.HyperConvergedSpec{},
	}
}

func newReq(inst *hcov1alpha1.HyperConverged) *hcoRequest {
	return &hcoRequest{
		Request:    request,
		logger:     log,
		conditions: newHcoConditions(),
		ctx:        context.TODO(),
		instance:   inst,
	}
}

func getBasicDeployment() *basicExpected {

	res := &basicExpected{}

	hco := &hcov1alpha1.HyperConverged{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: hcov1alpha1.HyperConvergedSpec{},
		Status: hcov1alpha1.HyperConvergedStatus{
			Conditions: []conditionsv1.Condition{
				{
					Type:    hcov1alpha1.ConditionReconcileComplete,
					Status:  corev1.ConditionTrue,
					Reason:  reconcileCompleted,
					Message: reconcileCompletedMessage,
				},
			},
			Versions: hcov1alpha1.Versions{
				{Name: hcoVersionName, Version: version.Version},
			},
		},
	}
	res.hco = hco

	res.pc = newKubeVirtPriorityClass()
	// These are all of the objects that we expect to "find" in the client because
	// we already created them in a previous reconcile.
	expectedKVConfig := newKubeVirtConfigForCR(hco, namespace)
	expectedKVConfig.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/configmaps/%s", expectedKVConfig.Namespace, expectedKVConfig.Name)
	res.kvConfig = expectedKVConfig

	expectedKVStorageConfig := newKubeVirtStorageConfigForCR(hco, namespace)
	expectedKVStorageConfig.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/configmaps/%s", expectedKVStorageConfig.Namespace, expectedKVStorageConfig.Name)
	res.kvStorageConfig = expectedKVStorageConfig

	expectedKV := newKubeVirtForCR(hco, namespace)
	expectedKV.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/kubevirts/%s", expectedKV.Namespace, expectedKV.Name)
	expectedKV.Status.Conditions = []kubevirtv1.KubeVirtCondition{
		{
			Type:   kubevirtv1.KubeVirtConditionAvailable,
			Status: corev1.ConditionTrue,
		},
		{
			Type:   kubevirtv1.KubeVirtConditionProgressing,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   kubevirtv1.KubeVirtConditionDegraded,
			Status: corev1.ConditionFalse,
		},
	}
	res.kv = expectedKV

	expectedCDI := newCDIForCR(hco, UndefinedNamespace)
	expectedCDI.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/cdis/%s", expectedCDI.Namespace, expectedCDI.Name)
	expectedCDI.Status.Conditions = getGenericCompletedConditions()
	res.cdi = expectedCDI

	expectedCNA := newNetworkAddonsForCR(hco, UndefinedNamespace)
	expectedCNA.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/cnas/%s", expectedCNA.Namespace, expectedCNA.Name)
	expectedCNA.Status.Conditions = getGenericCompletedConditions()
	res.cna = expectedCNA

	expectedKVCTB := newKubeVirtCommonTemplateBundleForCR(hco, OpenshiftNamespace)
	expectedKVCTB.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/ctbs/%s", expectedKVCTB.Namespace, expectedKVCTB.Name)
	expectedKVCTB.Status.Conditions = getGenericCompletedConditions()
	res.kvCtb = expectedKVCTB

	expectedKVNLB := newKubeVirtNodeLabellerBundleForCR(hco, namespace)
	expectedKVNLB.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/nlb/%s", expectedKVNLB.Namespace, expectedKVNLB.Name)
	expectedKVNLB.Status.Conditions = getGenericCompletedConditions()
	res.kvNlb = expectedKVNLB

	expectedKVTV := newKubeVirtTemplateValidatorForCR(hco, namespace)
	expectedKVTV.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/tv/%s", expectedKVTV.Namespace, expectedKVTV.Name)
	expectedKVTV.Status.Conditions = getGenericCompletedConditions()
	res.kvTv = expectedKVTV

	expectedVMI := newVMImportForCR(hco, namespace)
	expectedVMI.ObjectMeta.SelfLink = fmt.Sprintf("/apis/v1/namespaces/%s/vmimportconfigs/%s", expectedVMI.Namespace, expectedVMI.Name)
	expectedVMI.Status.Conditions = getGenericCompletedConditions()
	res.vmi = expectedVMI

	kvMtAg := newKubeVirtMetricsAggregationForCR(hco, namespace)
	kvMtAg.Status.Conditions = getGenericCompletedConditions()
	res.kvMtAg = kvMtAg

	res.imsConfig = newIMSConfigForCR(hco, namespace)

	return res
}

func checkAvailability(hco *hcov1alpha1.HyperConverged, expected corev1.ConditionStatus) {
	found := false
	for _, cond := range hco.Status.Conditions {
		if cond.Type == conditionsv1.ConditionType(kubevirtv1.KubeVirtConditionAvailable) {
			found = true
			Expect(cond.Status).To(Equal(expected))
			break
		}
	}

	if !found {
		Fail(fmt.Sprintf(`Can't find 'Available' condition; %v`, hco.Status.Conditions))
	}
}

// returns the HCO after reconcile, and the returned requeue
func doReconcile(cl client.Client, hco *hcov1alpha1.HyperConverged) (*hcov1alpha1.HyperConverged, bool) {
	r := initReconciler(cl)

	r.ownVersion = os.Getenv(util.HcoKvIoVersionName)
	if r.ownVersion == "" {
		r.ownVersion = version.Version
	}

	res, err := r.Reconcile(request)
	Expect(err).To(BeNil())

	foundResource := &hcov1alpha1.HyperConverged{}
	Expect(
		cl.Get(context.TODO(),
			types.NamespacedName{Name: hco.Name, Namespace: hco.Namespace},
			foundResource),
	).To(BeNil())

	return foundResource, res.Requeue
}

func getGenericCompletedConditions() []conditionsv1.Condition {
	return []conditionsv1.Condition{
		{
			Type:   conditionsv1.ConditionAvailable,
			Status: corev1.ConditionTrue,
		},
		{
			Type:   conditionsv1.ConditionProgressing,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   conditionsv1.ConditionDegraded,
			Status: corev1.ConditionFalse,
		},
	}
}

func getGenericProgressingConditions() []conditionsv1.Condition {
	return []conditionsv1.Condition{
		{
			Type:   conditionsv1.ConditionAvailable,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   conditionsv1.ConditionProgressing,
			Status: corev1.ConditionTrue,
		},
		{
			Type:   conditionsv1.ConditionDegraded,
			Status: corev1.ConditionFalse,
		},
	}
}

func initReconciler(client client.Client) *ReconcileHyperConverged {
	// Setup Scheme for all resources
	s := scheme.Scheme
	for _, f := range []func(*runtime.Scheme) error{
		apis.AddToScheme,
		cdiv1alpha1.AddToScheme,
		networkaddons.AddToScheme,
		sspopv1.AddToScheme,
		vmimportv1.AddToScheme,
	} {
		Expect(f(s)).To(BeNil())
	}

	// Create a ReconcileHyperConverged object with the scheme and fake client
	return &ReconcileHyperConverged{client: client, scheme: s}
}
