// (C) Copyright IBM Corp. 2025,2026
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ibm-aiu/dra-driver-spyre/internal/handler"
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	flgs "github.com/ibm-aiu/dra-driver-spyre/pkg/flags"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coreclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const templatePath = "../../assets"

var (
	testEnv         *envtest.Environment
	cfg             *rest.Config
	cdiHandler      *handler.CDIHandler
	cpManager       checkpointmanager.CheckpointManager
	cdiRoot         string
	configHostPath  string
	metricsHostPath string
	checkpointDir   string
)

func TestState(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "State Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testEnv = &envtest.Environment{}
	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())

	// temp directories
	cdiRoot = mkdirTemp("cdiroot")
	configHostPath = mkdirTemp("config-hostpath")
	metricsHostPath = mkdirTemp("metrics-hostpath")
	checkpointDir = mkdirTemp("checkpoint")

	// env vars expected by CDIHandler / config handler
	os.Setenv(cst.PseudoDeviceModeKey, cst.ModeEnabledValue)
	os.Setenv(cst.ConfigHostPathKey, configHostPath)
	os.Setenv(cst.MetricsHostPathKey, metricsHostPath)
	os.Setenv(cst.TemplatePathKey, templatePath)

	coreclient, err := coreclientset.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred())

	config := &flgs.Config{
		Flags: &flgs.Flags{
			LoggingConfig: flgs.NewLoggingConfig(),
			CDIRoot:       cdiRoot,
		},
		Coreclient: coreclient,
	}

	cdiHandler, err = handler.NewCDIHandler(config)
	Expect(err).NotTo(HaveOccurred())

	cpManager, err = checkpointmanager.NewCheckpointManager(checkpointDir)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	os.RemoveAll(cdiRoot)
	os.RemoveAll(configHostPath)
	os.RemoveAll(metricsHostPath)
	os.RemoveAll(checkpointDir)
	Expect(testEnv.Stop()).To(Succeed())
})

func mkdirTemp(name string) string {
	path, err := os.MkdirTemp("", name)
	Expect(err).NotTo(HaveOccurred())
	abs, err := filepath.Abs(path)
	Expect(err).NotTo(HaveOccurred())
	return abs
}
