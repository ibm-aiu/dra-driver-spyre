// (C) Copyright IBM Corp.  2025,2026 IBM Corp.
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

package cdi_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/ibm-aiu/dra-driver-spyre/internal/handler"
	conf "github.com/ibm-aiu/dra-driver-spyre/internal/handler/config"
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	"github.com/ibm-aiu/dra-driver-spyre/pkg/flags"
	flgs "github.com/ibm-aiu/dra-driver-spyre/pkg/flags"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coreclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var cdiHandler *CDIHandler
var configHandler *conf.ConfigHandler
var testEnv *envtest.Environment
var cfg *rest.Config
var CDIRoot string
var ConfigHostPath, MetricsHostPath string

const (
	TemplatePath = "../../assets"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Device Handler Suite")
}

func newConfig(cfg *rest.Config) *flags.Config {
	var err error
	CDIRoot, err = os.MkdirTemp("", "cdiroot")
	Expect(err).To(BeNil())
	coreclient, err := coreclientset.NewForConfig(cfg)
	Expect(err).To(BeNil())
	flags := &flags.Flags{
		LoggingConfig: flgs.NewLoggingConfig(),
		CDIRoot:       CDIRoot,
	}
	err = os.MkdirAll(CDIRoot, os.ModePerm)
	Expect(err).To(BeNil())
	ConfigHostPath = mkDirTemp("config-hostpath")
	MetricsHostPath = mkDirTemp("metrics-hostpath")
	os.Setenv(cst.ConfigHostPathKey, ConfigHostPath)
	os.Setenv(cst.MetricsHostPathKey, MetricsHostPath)
	os.Setenv(cst.TemplatePathKey, TemplatePath)
	return &flgs.Config{
		Flags:      flags,
		Coreclient: coreclient,
	}
}

func mkDirTemp(name string) string {
	path, err := os.MkdirTemp("", name)
	Expect(err).To(BeNil())
	absPath, err := filepath.Abs(path)
	Expect(err).To(BeNil())
	return absPath
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	testEnv = &envtest.Environment{}
	cfg, _ = testEnv.Start()
	config := newConfig(cfg)
	var err error
	cdiHandler, err = NewCDIHandler(config)
	Expect(err).To(BeNil())
	Expect(cdiHandler).NotTo(BeNil())
	configHandler, err = conf.InitConfigMount()
	Expect(err).To(BeNil())
})

var _ = AfterSuite(func() {
	os.RemoveAll(CDIRoot)
	os.RemoveAll(ConfigHostPath)
	os.RemoveAll(MetricsHostPath)
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
