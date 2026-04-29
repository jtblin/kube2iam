---
name: kube2iam-triage
description: Project-specific triage and design principles for kube2iam, focusing on AWS API limits and local environment support.
---

# Kube2iam Triage & Design Principles

This skill defines the technical constraints and design patterns specific to the `kube2iam` project.

## When to use this skill

- Use this when triaging issues related to AWS credentials, IAM roles, or STS.
- Use this when designing features that interact with host networking or iptables.
- Use this to address compatibility with local development tools like Minikube.

## How to use it

### 1. Be Mindful of External API Limits
- **Principle**: Stability of the AWS account takes precedence over minor latency gains.
- **Avoid**: Do not implement "prefetching" of credentials or high-frequency background refreshes that could lead to account-wide AWS STS/IAM throttling.
- **Design**: Design solutions that minimize the frequency and volume of AWS API calls.

### 2. Local Environment & Mocking Support
- **Principle**: Ensure the tool can run in mock or local environments where cloud services are unavailable.
- **Strategy**: Implement flags (e.g., `--disable-metadata-healthcheck`) to allow bypassing cloud-specific checks when static credentials are provided.

### 3. Environmental Troubleshooting
- **Clock Sync**: For `SignatureDoesNotMatch` errors, always investigate clock drift in the host/VM first.
- **Race Conditions**: Recognize that CNI interfaces may not be ready at startup. Implement graceful retry logic (e.g., 10-15s) in networking setup instead of immediate failure.
