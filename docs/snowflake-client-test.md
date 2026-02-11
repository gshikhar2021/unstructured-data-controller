# Unstructured Data Controller - Complete Setup Guide

This guide walks you through setting up the Unstructured Data Controller from scratch.

## Table of Contents
1. [Prerequisites](#prerequisites)
2. [Local Development Setup](#local-development-setup)
3. [Snowflake Setup](#snowflake-setup)
4. [Kubernetes Cluster Setup](#kubernetes-cluster-setup)
5. [Deploy Controller](#deploy-controller)
6. [Testing & Verification](#testing--verification)
7. [Troubleshooting](#troubleshooting)

---

## Prerequisites

Before starting, ensure you have the following installed:

- **Docker** (for Kind)
- **kubectl** (Kubernetes CLI)
- **kind** (Kubernetes in Docker)
- **awslocal** (LocalStack AWS CLI) or **aws** CLI
- **Snowflake Account** with appropriate permissions
- **Go 1.24+** (for building the controller)

### Environment Variables

Set the following environment variable:
```bash
export SNOWFLAKE_TARGET=<your-snowflake-environment>  # e.g., rhplatformtest
```

---

## Local Development Setup

### 1. Create Kind Cluster

```bash
kind create cluster --name unstructured-data-controller
```

### 2. Create Namespace

```bash
kubectl create namespace unstructured-controller-namespace
```

---

## Snowflake Setup

Run these commands in your Snowflake SQL worksheet:

```sql
USE WAREHOUSE DEFAULT;
USE DATABASE SNOWPIPE_DB;

-- Create schema
CREATE SCHEMA IF NOT EXISTS TESTUNSTRUCTUREDDATAPRODUCT;

-- Create stage with JSON file format
CREATE OR REPLACE STAGE SNOWPIPE_DB.TESTUNSTRUCTUREDDATAPRODUCT.TESTUNSTRUCTUREDDATAPRODUCT_INTERNAL_STG
    FILE_FORMAT = (TYPE = 'JSON');

-- Create role
CREATE ROLE IF NOT EXISTS TESTUNSTRUCTUREDDATAPRODUCT_SNOWPIPE_ROLE;

-- Grant permissions
GRANT USAGE ON DATABASE SNOWPIPE_DB TO ROLE TESTUNSTRUCTUREDDATAPRODUCT_SNOWPIPE_ROLE;
GRANT USAGE ON SCHEMA SNOWPIPE_DB.TESTUNSTRUCTUREDDATAPRODUCT TO ROLE TESTUNSTRUCTUREDDATAPRODUCT_SNOWPIPE_ROLE;
GRANT READ, WRITE ON STAGE SNOWPIPE_DB.TESTUNSTRUCTUREDDATAPRODUCT.TESTUNSTRUCTUREDDATAPRODUCT_INTERNAL_STG TO ROLE TESTUNSTRUCTUREDDATAPRODUCT_SNOWPIPE_ROLE;

-- Grant role to the service account user
GRANT ROLE TESTUNSTRUCTUREDDATAPRODUCT_SNOWPIPE_ROLE TO USER TESTUNSTRUCTUREDDATAPRODUCT_SNOWPIPE_PLATFORMTEST_APPUSER;
```

---

## Kubernetes Cluster Setup

### 1. Create AWS Secret

Create or update `config/samples/aws-secret.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aws-secret
  namespace: unstructured-controller-namespace
type: Opaque
stringData:
  AWS_REGION: us-west-2
  AWS_ACCESS_KEY_ID: LSIAQAAAAAAVNCBMPNSG
  AWS_SECRET_ACCESS_KEY: LSIAQAAAAAAVNCBMPNSG
  AWS_SESSION_TOKEN: LSIAQAAAAAAVNCBMPNSG
  AWS_ENDPOINT: http://localstack:4566  # For LocalStack
```

Apply the secret:
```bash
kubectl apply -f config/samples/aws-secret.yaml
```

### 2. Create Snowflake Private Key Secret

```bash
kubectl create secret generic rhplatformtest-private-key \
  -n unstructured-controller-namespace \
  --from-file=privateKey=/path/to/your/rsa_key.p8
```

**Note**: Replace `/path/to/your/rsa_key.p8` with the actual path to your Snowflake private key file.

Verify secrets:
```bash
kubectl get secrets -n unstructured-controller-namespace
```

### 3. Apply ControllerConfig CR

Update `config/samples/operator_v1alpha1_controllerconfig.yaml` with your settings, then apply:

```bash
kubectl apply -f config/samples/operator_v1alpha1_controllerconfig.yaml
```

Wait for the ControllerConfig to be ready:
```bash
kubectl get controllerconfig -n unstructured-controller-namespace -w
```

### 4. Apply SQSConsumer CR

```bash
kubectl apply -f config/samples/operator_v1alpha1_sqsconsumer.yaml
```

Verify:
```bash
kubectl get sqsconsumer -n unstructured-controller-namespace
```

### 5. Apply UnstructuredDataProduct CR

```bash
kubectl apply -f config/samples/operator_v1alpha1_unstructureddataproduct.yaml
```

Verify:
```bash
kubectl get unstructureddataproduct -n unstructured-controller-namespace
kubectl describe unstructureddataproduct testunstructureddataproduct -n unstructured-controller-namespace
```

---

## Deploy Controller

### Option 1: Run Locally (Development)

```bash
# Set environment variable
export SNOWFLAKE_TARGET=rhplatformtest

# Run the controller
make run
```

---

## Testing & Verification

### 1. Upload Test Files to S3

Using LocalStack (awslocal):

```bash
# Create bucket if not exists
awslocal s3 mb s3://data-ingestion-bucket

# Upload test PDF file
awslocal s3 cp /path/to/test.pdf s3://data-ingestion-bucket/testunstructureddataproduct/

# Verify upload
awslocal s3 ls s3://data-ingestion-bucket/testunstructureddataproduct/
```


### 3. Verify Files in Snowflake Stage

```sql
USE ROLE TESTUNSTRUCTUREDDATAPRODUCT_SNOWPIPE_ROLE;

-- List files in the stage
LIST @SNOWPIPE_DB.TESTUNSTRUCTUREDDATAPRODUCT.TESTUNSTRUCTUREDDATAPRODUCT_INTERNAL_STG;

-- Check file contents
SELECT $1 AS data
FROM @SNOWPIPE_DB.TESTUNSTRUCTUREDDATAPRODUCT.TESTUNSTRUCTUREDDATAPRODUCT_INTERNAL_STG
LIMIT 1;
```

Expected output:
- Files should be listed with `.gz` extension
- SELECT should return complete, valid JSON data
- JSON should contain `convertedDocument` with `metadata` and `content` fields

### 4. Verify Local Cache

Check files in the local cache directory:

```bash
ls -lah /path/to/cache/directory/testunstructureddataproduct/
```

You should see:
- `*.pdf` - Original files
- `*.pdf-metadata.json` - File metadata
- `*.pdf-converted.json` - Converted documents
