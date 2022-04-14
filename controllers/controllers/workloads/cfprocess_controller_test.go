package workloads_test

import (
	"context"
	"errors"
	"strconv"
	"time"

	networkingv1alpha1 "code.cloudfoundry.org/korifi/controllers/apis/networking/v1alpha1"

	eiriniv1 "code.cloudfoundry.org/eirini-controller/pkg/apis/eirini/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	workloadsv1alpha1 "code.cloudfoundry.org/korifi/controllers/apis/workloads/v1alpha1"
	. "code.cloudfoundry.org/korifi/controllers/controllers/workloads"
	"code.cloudfoundry.org/korifi/controllers/controllers/workloads/fake"
	. "code.cloudfoundry.org/korifi/controllers/controllers/workloads/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	testNamespace      = "test-ns"
	testProcessGUID    = "test-process-guid"
	testProcessType    = "web"
	testProcessCommand = "test-process-command"
	testAppGUID        = "test-app-guid"
	testBuildGUID      = "test-build-guid"
	testPackageGUID    = "test-package-guid"
)

var _ = Describe("CFProcessReconciler Unit Tests", func() {
	var (
		fakeClient *fake.Client
		envBuilder *fake.EnvBuilder

		cfBuild   *workloadsv1alpha1.CFBuild
		cfProcess *workloadsv1alpha1.CFProcess
		cfApp     *workloadsv1alpha1.CFApp
		lrp       *eiriniv1.LRP
		routes    []networkingv1alpha1.CFRoute

		cfBuildError   error
		cfAppError     error
		cfProcessError error
		lrpError       error
		lrpListError   error
		routeListError error

		cfProcessReconciler *CFProcessReconciler
		ctx                 context.Context
		req                 ctrl.Request

		reconcileErr error
	)

	BeforeEach(func() {
		fakeClient = new(fake.Client)

		envBuilder = new(fake.EnvBuilder)

		cfApp = BuildCFAppCRObject(testAppGUID, testNamespace)
		cfAppError = nil
		cfBuild = BuildCFBuildObject(testBuildGUID, testNamespace, testPackageGUID, testAppGUID)
		UpdateCFBuildWithDropletStatus(cfBuild)
		cfBuildError = nil
		cfProcess = BuildCFProcessCRObject(testProcessGUID, testNamespace, testAppGUID, testProcessType, testProcessCommand)
		cfProcessError = nil

		lrp = nil
		lrpError = nil
		lrpListError = nil

		fakeClient.GetStub = func(_ context.Context, name types.NamespacedName, obj client.Object) error {
			// cast obj to find its kind
			switch obj := obj.(type) {
			case *workloadsv1alpha1.CFProcess:
				cfProcess.DeepCopyInto(obj)
				return cfProcessError
			case *workloadsv1alpha1.CFBuild:
				cfBuild.DeepCopyInto(obj)
				return cfBuildError
			case *workloadsv1alpha1.CFApp:
				cfApp.DeepCopyInto(obj)
				return cfAppError
			case *eiriniv1.LRP:
				if lrp != nil && lrpError == nil {
					lrp.DeepCopyInto(obj)
				}
				return lrpError
			default:
				panic("TestClient Get provided a weird obj")
			}
		}

		fakeClient.ListStub = func(ctx context.Context, list client.ObjectList, option ...client.ListOption) error {
			switch listObj := list.(type) {
			case *eiriniv1.LRPList:
				lrpList := eiriniv1.LRPList{Items: []eiriniv1.LRP{}}
				if lrp != nil {
					lrpList.Items = append(lrpList.Items, *lrp)
				}
				lrpList.DeepCopyInto(listObj)
				return lrpListError
			case *networkingv1alpha1.CFRouteList:
				routeList := networkingv1alpha1.CFRouteList{Items: routes}

				routeList.DeepCopyInto(listObj)
				return routeListError
			default:
				panic("TestClient Get provided a weird obj")
			}
		}

		// configure a CFProcessReconciler with the client
		Expect(workloadsv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())
		cfProcessReconciler = &CFProcessReconciler{
			Client:     fakeClient,
			Scheme:     scheme.Scheme,
			Log:        zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)),
			EnvBuilder: envBuilder,
		}
		ctx = context.Background()
		req = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: testNamespace,
				Name:      testProcessGUID,
			},
		}
	})
	JustBeforeEach(func() {
		_, reconcileErr = cfProcessReconciler.Reconcile(ctx, req)
	})

	It("suceeds", func() {
		Expect(reconcileErr).NotTo(HaveOccurred())
	})

	When("the CFApp is created with desired state stopped", func() {
		BeforeEach(func() {
			cfApp.Spec.DesiredState = workloadsv1alpha1.StoppedState
		})

		It("does not attempt to create any new LRPs", func() {
			Expect(fakeClient.CreateCallCount()).To(Equal(0), "Client.Create call count mismatch")
		})
	})

	When("the CFApp is updated from desired state STARTED to STOPPED", func() {
		BeforeEach(func() {
			cfApp.Spec.DesiredState = workloadsv1alpha1.StoppedState
			lrp = &eiriniv1.LRP{
				ObjectMeta: metav1.ObjectMeta{
					Name:         testProcessGUID,
					GenerateName: "",
					Namespace:    testNamespace,
					Labels: map[string]string{
						workloadsv1alpha1.CFProcessGUIDLabelKey: testProcessGUID,
					},
				},
				Spec: eiriniv1.LRPSpec{
					GUID:        testProcessGUID,
					ProcessType: testProcessType,
					AppName:     cfApp.Spec.Name,
					AppGUID:     testAppGUID,
					Image:       "test-image-ref",
					Instances:   0,
					MemoryMB:    100,
					DiskMB:      100,
					CPUWeight:   0,
				},
				Status: eiriniv1.LRPStatus{
					Replicas: 0,
				},
			}
		})

		It("deletes any existing LRPs for the CFApp", func() {
			Expect(fakeClient.DeleteCallCount()).To(Equal(1), "Client.Delete call count mismatch")
		})
	})

	When("the CFApp is started and there are existing routes matching", func() {
		const testPort = 1234

		BeforeEach(func() {
			cfApp.Spec.DesiredState = workloadsv1alpha1.StartedState
			lrpError = apierrors.NewNotFound(schema.GroupResource{}, "some-guid")

			routes = []networkingv1alpha1.CFRoute{
				{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Time{
							Time: time.Now(),
						},
					},
					Status: networkingv1alpha1.CFRouteStatus{
						Destinations: []networkingv1alpha1.Destination{
							{
								GUID: "some-other-guid",
								Port: testPort + 1000,
								AppRef: corev1.LocalObjectReference{
									Name: testAppGUID,
								},
								ProcessType: testProcessType,
								Protocol:    "http1",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Time{
							Time: time.Now().Add(-5 * time.Second),
						},
					},
					Status: networkingv1alpha1.CFRouteStatus{
						Destinations: []networkingv1alpha1.Destination{
							{
								GUID: "some-guid",
								Port: testPort,
								AppRef: corev1.LocalObjectReference{
									Name: testAppGUID,
								},
								ProcessType: testProcessType,
								Protocol:    "http1",
							},
						},
					},
				},
			}
		})

		It("builds the environment for the app", func() {
			Expect(envBuilder.BuildEnvCallCount()).To(Equal(1))
			_, actualApp := envBuilder.BuildEnvArgsForCall(0)
			Expect(actualApp).To(Equal(cfApp))
		})

		It("chooses the oldest matching route", func() {
			_, obj, _ := fakeClient.CreateArgsForCall(0)
			returnedLRP := obj.(*eiriniv1.LRP)
			Expect(returnedLRP.Spec.Env).To(HaveKeyWithValue("PORT", strconv.Itoa(testPort)))
			Expect(returnedLRP.Spec.Env).To(HaveKeyWithValue("VCAP_APP_PORT", strconv.Itoa(testPort)))
		})
	})

	When("the app is started", func() {
		BeforeEach(func() {
			cfApp.Spec.DesiredState = workloadsv1alpha1.StartedState
		})

		When("fetch CFProcess returns an error", func() {
			BeforeEach(func() {
				cfProcessError = errors.New(failsOnPurposeErrorMessage)
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(failsOnPurposeErrorMessage))
			})
		})

		When("fetch CFProcess returns a NotFoundError", func() {
			BeforeEach(func() {
				cfProcessError = apierrors.NewNotFound(schema.GroupResource{}, cfProcess.Name)
			})

			It("doesn't return an error", func() {
				Expect(reconcileErr).NotTo(HaveOccurred())
			})
		})

		When("fetch CFApp returns an error", func() {
			BeforeEach(func() {
				cfAppError = errors.New(failsOnPurposeErrorMessage)
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(failsOnPurposeErrorMessage))
			})
		})

		When("fetch CFBuild returns an error", func() {
			BeforeEach(func() {
				cfBuildError = errors.New(failsOnPurposeErrorMessage)
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(failsOnPurposeErrorMessage))
			})
		})

		When("CFBuild does not have a build droplet status", func() {
			BeforeEach(func() {
				cfBuild.Status.BuildDropletStatus = nil
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError("no build droplet status on CFBuild"))
			})
		})

		When("building the LRP environment fails", func() {
			BeforeEach(func() {
				envBuilder.BuildEnvReturns(nil, errors.New("build-env-err"))
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(ContainSubstring("build-env-err")))
			})
		})

		When("fetch LRPList returns an error", func() {
			BeforeEach(func() {
				lrpListError = errors.New(failsOnPurposeErrorMessage)
			})

			It("returns an error", func() {
				Expect(reconcileErr).To(MatchError(failsOnPurposeErrorMessage))
			})
		})
	})
})
