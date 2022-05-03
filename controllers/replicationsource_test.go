package controllers

import (
	"testing"

	volsyncv1alpha1 "github.com/backube/volsync/api/v1alpha1"
	"github.com/go-logr/logr"
	pvcv1alpha1 "github.com/konveyor/volume-snapshot-mover/api/v1alpha1"
	snapv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	aws_access_key_id     = "some_aws_access_key_id"
	aws_secret_access_key = "some_aws_secret_access_key"
	restic_password       = "some_restic_password"
	restic_repo           = "some_restic_repo"
	namespace             = "foo"
	resticSecret          = "restic-secret"
)

var (
	secretData = map[string][]byte{
		"data": []byte(
			"AWS_ACCESS_KEY_ID:" + aws_access_key_id + "\n" +
				"AWS_SECRET_ACCESS_KEY:" + aws_secret_access_key + "\n" +
				"RESTIC_PASSWORD:" + restic_password + "\n" +
				"RESTIC_REPOSITORY:" + restic_repo),
	}
)

func getSchemeForFakeClientRepSrc() (*runtime.Scheme, error) {
	err := pvcv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}
	err = snapv1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}
	err = volsyncv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}

	return scheme.Scheme, nil
}

func getFakeClientFromObjectsRepSrc(objs ...client.Object) (client.WithWatch, error) {
	schemeForFakeClient, err := getSchemeForFakeClientRepSrc()
	if err != nil {
		return nil, err
	}

	return fake.NewClientBuilder().WithScheme(schemeForFakeClient).WithObjects(objs...).Build(), nil
}

func TestDataMoverBackupReconciler_BuildReplicationSource(t *testing.T) {
	tests := []struct {
		name    string
		dmb     *pvcv1alpha1.DataMoverBackup
		pvc     *corev1.PersistentVolumeClaim
		repsrc  *volsyncv1alpha1.ReplicationSource
		secret  *corev1.Secret
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "given valid pvc,secret -> create valid rep src",
			dmb: &pvcv1alpha1.DataMoverBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmb",
					Namespace: "bar",
				},
				Spec: pvcv1alpha1.DataMoverBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: "foo",
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-pvc",
					Namespace: namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("10Gi"),
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmb-secret",
					Namespace: namespace,
				},
				Data: secretData,
			},
			repsrc: &volsyncv1alpha1.ReplicationSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmb-rep-src",
					Namespace: namespace,
				},
			},
			wantErr: false,
		},
		{
			name: "given invalid secret -> err",
			dmb: &pvcv1alpha1.DataMoverBackup{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmb",
					Namespace: "bar",
				},
				Spec: pvcv1alpha1.DataMoverBackupSpec{
					VolumeSnapshotContent: corev1.ObjectReference{
						Name: "sample-snapshot",
					},
					ProtectedNamespace: namespace,
				},
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: v1.ObjectMeta{
					Name:      "pvc",
					Namespace: namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("10Gi"),
						},
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmb",
					Namespace: namespace,
				},
				Data: secretData,
			},
			repsrc: &volsyncv1alpha1.ReplicationSource{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sample-dmb-rep-src",
					Namespace: namespace,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient, err := getFakeClientFromObjectsRepSrc(tt.dmb, tt.pvc, tt.secret)
			if err != nil {
				t.Errorf("error creating fake client, likely programmer error")
			}
			r := &DataMoverBackupReconciler{
				Client:  fakeClient,
				Scheme:  fakeClient.Scheme(),
				Log:     logr.Discard(),
				Context: newContextForTest(tt.name),
				NamespacedName: types.NamespacedName{
					Namespace: tt.dmb.Spec.ProtectedNamespace,
					Name:      tt.dmb.Name,
				},
				EventRecorder: record.NewFakeRecorder(10),
				req: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: tt.dmb.Namespace,
						Name:      tt.dmb.Name,
					},
				},
			}
			err = r.buildReplicationSource(tt.repsrc, tt.dmb, tt.pvc)
			if (err != nil) != tt.wantErr {
				t.Errorf("DataMoverBackupReconciler.buildReplicationSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}