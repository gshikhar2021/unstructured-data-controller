//go:build e2e
// +build e2e

/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/redhat-data-and-ai/unstructured-data-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
	"sigs.k8s.io/e2e-framework/support/utils"
)

var (
	testenv         env.Environment
	kindClusterName string
)

const (
	testNamespace       = "unstructured-controller-namespace"
	deploymentName      = "unstructured-data-controller-controller-manager"
	snowflakeSecretName = "snowflake-private-key"
)

func TestMain(m *testing.M) {
	testenv = env.New()
	runningProcesses := []exec.Cmd{}

	kindClusterName = fmt.Sprintf("test-cluster-%d", time.Now().UnixNano())
	skipClusterSetup := os.Getenv("SKIP_CLUSTER_SETUP")
	skipClusterCleanup := os.Getenv("SKIP_CLUSTER_CLEANUP")
	cleanupRequired := os.Getenv("SKIP_TEST_CLEANUP") != "true"

	testenv.Setup(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			kindCluster := kind.NewCluster(kindClusterName)

			if skipClusterSetup != "true" {
				log.Printf("Creating new kind cluster with name: %s", kindClusterName)
				envFuncs := []env.Func{
					envfuncs.CreateCluster(kindCluster, kindClusterName),
					envfuncs.CreateNamespace(testNamespace),
				}
				for _, envFunc := range envFuncs {
					if _, err := envFunc(ctx, config); err != nil {
						log.Fatalf("Failed to create kind cluster: %s", err)
					}
				}
			}

			if err := testSetup(ctx, &runningProcesses, config); err != nil {
				if cleanupRequired {
					_ = testCleanup(ctx, config, &runningProcesses)
				}
				log.Fatalf("failed to setup test environment: %s", err)
			}
			return ctx, nil
		},
	)

	testenv.Finish(
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			log.Println("finishing tests, cleaning cluster ...")
			if cleanupRequired {
				if err := testCleanup(ctx, cfg, &runningProcesses); err != nil {
					log.Printf("failed to cleanup test environment: %s", err)
					return ctx, err
				}
			}
			return ctx, nil
		},
	)

	if skipClusterCleanup != "true" && skipClusterSetup != "true" {
		testenv.Finish(
			envfuncs.DeleteNamespace(testNamespace),
			envfuncs.DestroyCluster(kindClusterName),
		)
	}

	os.Exit(testenv.Run(m))
}

func TestConfigHealthy(t *testing.T) {
	// Setup creates ControllerConfig and waits for ConfigReady=true; this test runs after setup.
	t.Log("ControllerConfig is healthy (validated in testSetup)")
}

func testSetup(ctx context.Context, runningProcesses *[]exec.Cmd, config *envconf.Config) error {
	// change dir for Makefile or it will fail
	if err := os.Chdir("../../"); err != nil {
		log.Printf("Unable to set working directory: %s", err)
		return err
	}

	image := os.Getenv("IMG")
	if image == "" {
		return fmt.Errorf("IMG environment variable is required")
	}

	log.Println("Deploying operator with CRDs installed...")
	deployCommand := fmt.Sprintf("make IMG=%s deploy", image)
	if p := utils.RunCommand(deployCommand); p.Err() != nil {
		log.Printf("Failed to deploy operator: %s: %s", p.Err(), p.Result())
		return p.Err()
	}

	log.Println("Patching controller-manager to add cache directory volume...")
	patchCommand := fmt.Sprintf(`kubectl patch deployment %s -n %s --type=json -p '[
	{
		"op": "add",
		"path": "/spec/template/spec/volumes/-",
		"value": 
		{
			"name": "cache-volume",
			"emptyDir": {}
		}
	},
	{
		"op": "add",
		"path": "/spec/template/spec/containers/0/volumeMounts/-",
		"value": {
			"name": "cache-volume",
			"mountPath": "/tmp/cache"
		}
	}
	]'`, deploymentName, testNamespace)

	if p := utils.RunCommand(patchCommand); p.Err() != nil {
		log.Printf("Failed to patch deployment: %s: %s", p.Err(), p.Result())
		return p.Err()
	}

	log.Println("Waiting for controller-manager deployment to be available...")
	client := config.Client()
	if err := wait.For(
		conditions.New(client.Resources()).DeploymentAvailable(deploymentName, testNamespace),
		wait.WithTimeout(10*time.Minute),
		wait.WithInterval(2*time.Second),
	); err != nil {
		log.Printf("Timed out waiting for deployment: %s", err)
		return err
	}

	log.Println("Capturing logs from controller-manager")
	logFile, err := os.Create("controller-manager-logs.txt")
	if err != nil {
		log.Printf("failed to create log file: %s", err)
	} else {
		kubectlLogs := exec.Command("kubectl", "logs", "-f", "-n", testNamespace, "deployments/"+deploymentName)
		kubectlLogs.Stdout = logFile
		kubectlLogs.Stderr = logFile
		if err := kubectlLogs.Start(); err == nil {
			*runningProcesses = append(*runningProcesses, *kubectlLogs)
		}
		logFile.Close()
	}

	log.Println("Creating snowflake secret with private key")
	secretFile := os.Getenv("SNOWFLAKE_SECRET_FILE")
	if secretFile == "" {
		return fmt.Errorf("SNOWFLAKE_SECRET_FILE environment variable is required for snowflake secret")
	}
	secretCreateCmd := fmt.Sprintf("kubectl create secret generic %s -n %s --from-file=privateKey=%s",
		snowflakeSecretName, testNamespace, secretFile)
	if p := utils.RunCommand(secretCreateCmd); p.Err() != nil {
		log.Printf("Failed to create snowflake secret: %s %s", p.Err(), p.Result())
		return p.Err()
	}

	log.Println("Creating aws-secret from config/samples/aws-secret.yaml")
	if p := utils.RunCommand(fmt.Sprintf("kubectl apply -n %s -f config/samples/aws-secret.yaml", testNamespace)); p.Err() != nil {
		log.Printf("Failed to create aws-secret: %s %s", p.Err(), p.Result())
		return p.Err()
	}

	skipLocalstack := os.Getenv("SKIP_LOCALSTACK_SETUP")
	if skipLocalstack != "true" {
		log.Println("Deploying localstack...")
		if p := utils.RunCommand(fmt.Sprintf("kubectl apply -n %s -f test/localstack/", testNamespace)); p.Err() != nil {
			log.Printf("Failed to deploy localstack: %s %s", p.Err(), p.Result())
			return p.Err()
		}
		log.Println("Waiting for localstack to be ready...")
		if err := wait.For(
			conditions.New(client.Resources()).DeploymentAvailable("localstack", testNamespace),
			wait.WithTimeout(10*time.Minute),
			wait.WithInterval(5*time.Second),
		); err != nil {
			log.Printf("Timed out waiting for localstack: %s", err)
			return err
		}
		log.Println("Port-forwarding localstack to localhost:4566")
		pf := exec.Command("kubectl", "port-forward", "-n", testNamespace, "services/localstack", "4566:4566")
		pf.Stdout = os.Stdout
		pf.Stderr = os.Stderr
		if err := pf.Start(); err != nil {
			log.Printf("failed to port-forward localstack: %s", err)
			return err
		}
		*runningProcesses = append(*runningProcesses, *pf)
	}

	skipDocling := os.Getenv("SKIP_DOCLING_SETUP")
	if skipDocling != "true" {
		log.Println("Deploying docling-serve...")
		if p := utils.RunCommand(fmt.Sprintf("kubectl apply -n %s -f test/docling-serve/", testNamespace)); p.Err() != nil {
			log.Printf("Failed to deploy docling-serve: %s %s", p.Err(), p.Result())
			return p.Err()
		}
		log.Println("Waiting for docling-serve to be ready...")
		if err := wait.For(
			conditions.New(client.Resources()).DeploymentAvailable("docling-serve", testNamespace),
			wait.WithTimeout(10*time.Minute),
			wait.WithInterval(5*time.Second),
		); err != nil {
			log.Printf("Timed out waiting for docling-serve: %s", err)
			return err
		}
		log.Println("Port-forwarding docling-serve to localhost:5002")
		pf := exec.Command("kubectl", "port-forward", "-n", testNamespace, "services/docling-serve", "5002:5001")
		pf.Stdout = os.Stdout
		pf.Stderr = os.Stderr
		if err := pf.Start(); err != nil {
			log.Printf("failed to port-forward docling-serve: %s", err)
			return err
		}
		*runningProcesses = append(*runningProcesses, *pf)
	}

	log.Println("Creating ControllerConfig CR...")
	if err := v1alpha1.AddToScheme(client.Resources(testNamespace).GetScheme()); err != nil {
		return err
	}
	configCR := getControllerConfigResource()
	if err := client.Resources().Create(ctx, configCR); err != nil {
		log.Printf("failed to create ControllerConfig: %s", err)
		return err
	}

	log.Println("Waiting for ControllerConfig to be healthy (ConfigReady=true)...")
	configWaitCmd := fmt.Sprintf(
		"kubectl wait --for=condition=ConfigReady=true controllerconfigs.operator.dataverse.redhat.com/controllerconfig -n %s --timeout=2m",
		testNamespace,
	)
	if p := utils.RunCommand(configWaitCmd); p.Err() != nil {
		log.Printf("failed to meet condition for ControllerConfig: %s %s", p.Err(), p.Result())
		return p.Err()
	}
	log.Println("ControllerConfig is healthy")
	return nil
}

func getControllerConfigResource() *v1alpha1.ControllerConfig {
	account := os.Getenv("ACCOUNT")
	user := os.Getenv("USER")
	role := os.Getenv("ROLE")
	warehouse := os.Getenv("WAREHOUSE")
	if account == "" {
		account = "gdadclc-rhplatformtest"
	}
	if user == "" {
		user = "shikgupt"
	}
	if role == "" {
		role = "accountadmin"
	}
	if warehouse == "" {
		warehouse = "DEFAULT"
	}
	return &v1alpha1.ControllerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controllerconfig",
			Namespace: testNamespace,
		},
		Spec: v1alpha1.ControllerConfigSpec{
			AWSSecret: "aws-secret",
			SnowflakeConfig: v1alpha1.SnowflakeConfig{
				Name:             "e2e",
				Account:          account,
				User:             user,
				Role:             role,
				Region:           "us-west-2",
				Warehouse:        warehouse,
				PrivateKeySecret: snowflakeSecretName,
			},
			UnstructuredDataProcessingConfig: v1alpha1.UnstructuredDataProcessingConfigSpec{
				DoclingServeURL:             "http://docling-serve:5001",
				IngestionBucket:             "unstructured-bucket",
				DataStorageBucket:           "data-storage-bucket",
				CacheDirectory:              "/tmp/cache/",
				MaxConcurrentDoclingTasks:   5,
				MaxConcurrentLangchainTasks: 10,
			},
		},
	}
}

func testCleanup(ctx context.Context, cfg *envconf.Config, runningProcesses *[]exec.Cmd) error {
	log.Println("cleaning up test environment ...")
	errorList := []error{}

	cleanupResources(ctx, cfg, testNamespace)

	commandList := []string{
		"make undeploy ignore-not-found=true",
		fmt.Sprintf("kubectl delete secret %s -n %s --ignore-not-found=true", snowflakeSecretName, testNamespace),
		fmt.Sprintf("kubectl delete secret aws-secret -n %s --ignore-not-found=true", testNamespace),
		fmt.Sprintf("kubectl delete controllerconfigs.operator.dataverse.redhat.com controllerconfig -n %s --ignore-not-found=true", testNamespace),
		fmt.Sprintf("kubectl delete -f test/localstack/ -n %s --ignore-not-found=true", testNamespace),
		fmt.Sprintf("kubectl delete -f test/docling-serve/ -n %s --ignore-not-found=true", testNamespace),
	}
	for _, command := range commandList {
		if p := utils.RunCommand(command); p.Err() != nil {
			errorList = append(errorList, fmt.Errorf("failed to run command: %s: %s", command, p.Err()))
		}
	}

	for _, process := range *runningProcesses {
		if killErr := process.Process.Kill(); killErr != nil {
			errorList = append(errorList, killErr)
		}
	}

	if len(errorList) > 0 {
		return fmt.Errorf("failed to cleanup test environment: %v", errorList)
	}
	return nil
}

// cleanupResources performs cleanup of all CRs used in e2e tests before operator deletion to prevent orphaned resources and namespace cleanup issues
func cleanupResources(ctx context.Context, cfg *envconf.Config, namespace string) (context.Context, error) {
	log.Println("Cleaning up all remaining CRs before operator deletion...")

	// Delete all CRs used in e2e tests with timeout to prevent orphaned resources
	log.Println("Deleting all resources from the namespace...")
	utils.RunCommand(fmt.Sprintf(`kubectl api-resources --verbs=list --namespaced -o name | grep operator.dataverse.redhat.com | xargs -n 1 kubectl delete --all --ignore-not-found -n %s --timeout=60s`, namespace))

	// Verify cleanup completion and log remaining CRs
	log.Println("Verifying CR cleanup completion...")
	p := utils.RunCommand(fmt.Sprintf(`kubectl api-resources --verbs=list --namespaced -o name | grep operator.dataverse.redhat.com | xargs -n 1 kubectl get --ignore-not-found -n %s 2>/dev/null || echo "All CRs successfully deleted"`, namespace))

	if p.Err() != nil {
		log.Printf("Verification command failed: %s", p.Err())
	} else {
		output := p.Result()
		if output == "All CRs successfully deleted" {
			log.Println("All CRs successfully deleted")
		} else {
			log.Printf("Remaining CRs found, you need to delete them manually from the cluster:\n%s", output)
		}
	}

	return ctx, nil
}
