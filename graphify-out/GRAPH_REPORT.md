# Graph Report - .  (2026-05-03)

## Corpus Check
- Corpus is ~22,271 words - fits in a single context window. You may not need a graph.

## Summary
- 306 nodes · 489 edges · 32 communities detected
- Extraction: 83% EXTRACTED · 17% INFERRED · 0% AMBIGUOUS · INFERRED: 81 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_AWS STS Client|AWS STS Client]]
- [[_COMMUNITY_IAM & ARN Logic|IAM & ARN Logic]]
- [[_COMMUNITY_Namespace Handlers|Namespace Handlers]]
- [[_COMMUNITY_Mock STSRegion Clients|Mock STS/Region Clients]]
- [[_COMMUNITY_IMDS & STS Mocks|IMDS & STS Mocks]]
- [[_COMMUNITY_CLI & Main Loop|CLI & Main Loop]]
- [[_COMMUNITY_K8s Client Utilities|K8s Client Utilities]]
- [[_COMMUNITY_E2E DaemonSet Tests|E2E DaemonSet Tests]]
- [[_COMMUNITY_Role Mapper Tests|Role Mapper Tests]]
- [[_COMMUNITY_K8s API Listing|K8s API Listing]]
- [[_COMMUNITY_IAM Role Mapper|IAM Role Mapper]]
- [[_COMMUNITY_IPTables Logic|IPTables Logic]]
- [[_COMMUNITY_Pod Event Handlers|Pod Event Handlers]]
- [[_COMMUNITY_K8s Store Mocks|K8s Store Mocks]]
- [[_COMMUNITY_Helm Deployment|Helm Deployment]]
- [[_COMMUNITY_Kustomize DaemonSet Base|Kustomize DaemonSet Base]]
- [[_COMMUNITY_Project Documentation|Project Documentation]]
- [[_COMMUNITY_Kustomize Overlays|Kustomize Overlays]]
- [[_COMMUNITY_E2E Kind Config|E2E Kind Config]]
- [[_COMMUNITY_Namespace Logic Tests|Namespace Logic Tests]]
- [[_COMMUNITY_Pod Data Model|Pod Data Model]]
- [[_COMMUNITY_Version Metadata|Version Metadata]]
- [[_COMMUNITY_Helm ServiceMonitor|Helm ServiceMonitor]]
- [[_COMMUNITY_Helm PSP|Helm PSP]]
- [[_COMMUNITY_Helm Notes|Helm Notes]]
- [[_COMMUNITY_Helm Service|Helm Service]]
- [[_COMMUNITY_Helm RBAC ClusterRole|Helm RBAC ClusterRole]]
- [[_COMMUNITY_Helm ServiceAccount|Helm ServiceAccount]]
- [[_COMMUNITY_Helm RBAC Binding|Helm RBAC Binding]]
- [[_COMMUNITY_Kustomize PSP Overlay|Kustomize PSP Overlay]]
- [[_COMMUNITY_Kustomize RBAC Base|Kustomize RBAC Base]]
- [[_COMMUNITY_E2E Test Data|E2E Test Data]]

## God Nodes (most connected - your core abstractions)
1. `Server` - 15 edges
2. `newLogger()` - 14 edges
3. `NewServer()` - 13 edges
4. `newPodIndexer()` - 10 edges
5. `newNamespaceIndexer()` - 10 edges
6. `newTestClient()` - 10 edges
7. `buildServer()` - 10 edges
8. `NewPodHandler()` - 9 edges
9. `Client` - 9 edges
10. `newRoleMapper()` - 9 edges

## Surprising Connections (you probably didn't know these)
- `EKS Configuration Example` --semantically_similar_to--> `DaemonSet Template`  [INFERRED] [semantically similar]
  examples/eks-example.yml → charts/kube2iam/templates/daemonset.yaml
- `TestGetNamespaceRoleAnnotation()` --calls--> `GetNamespaceRoleAnnotation()`  [INFERRED]
  namespace_test.go → namespace.go
- `TestGetNamespaceRoleAnnotationMissingKey()` --calls--> `GetNamespaceRoleAnnotation()`  [INFERRED]
  namespace_test.go → namespace.go
- `TestGetNamespaceRoleAnnotationNilAnnotations()` --calls--> `GetNamespaceRoleAnnotation()`  [INFERRED]
  namespace_test.go → namespace.go
- `TestNamespaceHandlerOnAdd()` --calls--> `NewNamespaceHandler()`  [INFERRED]
  namespace_test.go → namespace.go

## Hyperedges (group relationships)
- **Core Proxy Logic** — server_server, iam_iam, k8s_k8s, mapper_mapper [INFERRED 0.85]
- **Application Initialization Flow** — main_main, iptables_iptables, version_version [INFERRED 0.75]
- **Helm Chart Deployment Pattern** — chart_kube2iam, values_kube2iam, daemonset_template, clusterrole_template [EXTRACTED 1.00]
- **Kustomize Deployment Pattern** — kustomization_base, kustomization_overlay, daemonset_overlay [EXTRACTED 1.00]
- **E2E Test Environment** — kind_config, kube2iam_mocks, kube2iam_ds_testdata [INFERRED 0.95]

## Communities

### Community 0 - "AWS STS Client"
Cohesion: 0.08
Nodes (34): Client, Credentials, GetEndpointFromRegion(), getHash(), getIAMCode(), GetInstanceIAMRole(), getMetadataPath(), IsValidRegion() (+26 more)

### Community 1 - "IAM & ARN Logic"
Cohesion: 0.07
Nodes (22): ARN Parsing Logic, IAM Provider Interface, mockAPIError, IPTables Management, Kubernetes Client Interface, Application Entrypoint, Role Mapper, Prometheus Metrics (+14 more)

### Community 2 - "Namespace Handlers"
Cohesion: 0.09
Nodes (29): GetNamespaceRoleAnnotation(), NamespaceIndexFunc(), NewNamespaceHandler(), TestGetNamespaceRoleAnnotation(), TestGetNamespaceRoleAnnotationMissingKey(), TestGetNamespaceRoleAnnotationNilAnnotations(), TestIsPodActive(), TestNamespaceHandlerOnAdd() (+21 more)

### Community 3 - "Mock STS/Region Clients"
Cohesion: 0.19
Nodes (24): mockRegionClient, mockSTSClient, NewServer(), buildServer(), newLogger(), newRoleMapper(), newTestIAMClient(), setMuxVars() (+16 more)

### Community 4 - "IMDS & STS Mocks"
Cohesion: 0.1
Nodes (14): countingSTSClient, integMockIMDS, integrationIAMClient(), newIntegServer(), TestIntegErrorCachingCurrentBehavior(), TestIntegFullRequestChain(), TestIntegHealthcheck(), TestIntegNamespaceRestrictionAllowed() (+6 more)

### Community 5 - "CLI & Main Loop"
Cohesion: 0.17
Nodes (12): addFlags(), main(), GetBaseArn(), GetBaseArnWithClient(), IsValidBaseARN(), TestGetBaseArnWithClient(), TestGetBaseArnWithClientIMDSError(), TestGetBaseArnWithClientMalformedARN() (+4 more)

### Community 6 - "K8s Client Utilities"
Cohesion: 0.44
Nodes (14): newNamespaceIndexer(), newPodIndexer(), newTestClient(), runningPod(), TestListNamespaces(), TestListPodIPs(), TestListPodIPsEmpty(), TestListPodIPsExcludesInactivePods() (+6 more)

### Community 7 - "E2E DaemonSet Tests"
Cohesion: 0.2
Nodes (11): TestDaemonSetScheduledOnAllNodes(), TestIPTablesRulesInstalled(), buildKubeClient(), dumpDebugInfo(), execInPod(), getDaemonSet(), kubectlApply(), loadImageIntoKind() (+3 more)

### Community 8 - "Role Mapper Tests"
Cohesion: 0.14
Nodes (1): storeMock

### Community 9 - "K8s API Listing"
Cohesion: 0.21
Nodes (2): Client, resolveDuplicatedIP()

### Community 10 - "IAM Role Mapper"
Cohesion: 0.28
Nodes (3): RoleMapper, RoleMappingResult, store

### Community 11 - "IPTables Logic"
Cohesion: 0.36
Nodes (5): AddRule(), checkInterfaceExists(), TestCheckInterfaceExistsFailsWithBogusInterface(), TestCheckInterfaceExistsPassesWithPlus(), TestCheckInterfaceExistsPassesWithValidInterface()

### Community 12 - "Pod Event Handlers"
Cohesion: 0.7
Nodes (1): PodHandler

### Community 13 - "K8s Store Mocks"
Cohesion: 0.4
Nodes (1): mockStore

### Community 14 - "Helm Deployment"
Cohesion: 0.4
Nodes (5): Kube2iam Helm Chart, DaemonSet Template, EKS Configuration Example, Helm Chart Documentation, Helm Chart Values

### Community 15 - "Kustomize DaemonSet Base"
Cohesion: 0.67
Nodes (3): Kustomize Base DaemonSet, PSP Calico Overlay DaemonSet, PSP Calico Overlay README

### Community 16 - "Project Documentation"
Cohesion: 1.0
Nodes (2): Kubernetes Deployment Manifest, Project Documentation

### Community 17 - "Kustomize Overlays"
Cohesion: 1.0
Nodes (2): Kustomize Base Configuration, PSP Calico Kustomization

### Community 18 - "E2E Kind Config"
Cohesion: 1.0
Nodes (2): Kind Cluster E2E Config, IAM Mocks for E2E Tests

### Community 19 - "Namespace Logic Tests"
Cohesion: 1.0
Nodes (1): Namespace Tests

### Community 20 - "Pod Data Model"
Cohesion: 1.0
Nodes (1): Pod Structure

### Community 21 - "Version Metadata"
Cohesion: 1.0
Nodes (1): Version Information

### Community 22 - "Helm ServiceMonitor"
Cohesion: 1.0
Nodes (1): ServiceMonitor Template

### Community 23 - "Helm PSP"
Cohesion: 1.0
Nodes (1): PodSecurityPolicy Template

### Community 24 - "Helm Notes"
Cohesion: 1.0
Nodes (1): Helm Installation Notes

### Community 25 - "Helm Service"
Cohesion: 1.0
Nodes (1): Service Template

### Community 26 - "Helm RBAC ClusterRole"
Cohesion: 1.0
Nodes (1): ClusterRole Template

### Community 27 - "Helm ServiceAccount"
Cohesion: 1.0
Nodes (1): ServiceAccount Template

### Community 28 - "Helm RBAC Binding"
Cohesion: 1.0
Nodes (1): ClusterRoleBinding Template

### Community 29 - "Kustomize PSP Overlay"
Cohesion: 1.0
Nodes (1): PSP Calico Overlay PodSecurityPolicy

### Community 30 - "Kustomize RBAC Base"
Cohesion: 1.0
Nodes (1): Kustomize Base RBAC

### Community 31 - "E2E Test Data"
Cohesion: 1.0
Nodes (1): E2E DaemonSet Test Data

## Knowledge Gaps
- **37 isolated node(s):** `appHandlerFunc`, `HealthResponse`, `STSClient`, `RegionClient`, `IMDSClient` (+32 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Role Mapper Tests`** (14 nodes): `mapper_test.go`, `TestCheckRoleForNamespace()`, `TestDumpDebugInfo()`, `TestExtractRoleARN()`, `TestGetExternalIDMappingWithAnnotation()`, `TestGetExternalIDMappingWithoutAnnotation()`, `TestGetRoleMappingNoAnnotationNoDefault()`, `TestGetRoleMappingPodNotFound()`, `TestGetRoleMappingWithDefault()`, `storeMock`, `.ListNamespaces()`, `.ListPodIPs()`, `.NamespaceByName()`, `.PodByIP()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `K8s API Listing`** (12 nodes): `Client`, `.createNamespaceLW()`, `.createPodLW()`, `.ListNamespaces()`, `.ListPodIPs()`, `.NamespaceByName()`, `.PodByIP()`, `.WatchForNamespaces()`, `.WatchForPods()`, `k8s.go`, `NewClient()`, `resolveDuplicatedIP()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Pod Event Handlers`** (5 nodes): `PodHandler`, `.OnAdd()`, `.OnDelete()`, `.OnUpdate()`, `.podFields()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `K8s Store Mocks`** (5 nodes): `mockStore`, `.ListNamespaces()`, `.ListPodIPs()`, `.NamespaceByName()`, `.PodByIP()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Project Documentation`** (2 nodes): `Kubernetes Deployment Manifest`, `Project Documentation`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Kustomize Overlays`** (2 nodes): `Kustomize Base Configuration`, `PSP Calico Kustomization`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `E2E Kind Config`** (2 nodes): `Kind Cluster E2E Config`, `IAM Mocks for E2E Tests`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Namespace Logic Tests`** (1 nodes): `Namespace Tests`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Pod Data Model`** (1 nodes): `Pod Structure`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Version Metadata`** (1 nodes): `Version Information`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Helm ServiceMonitor`** (1 nodes): `ServiceMonitor Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Helm PSP`** (1 nodes): `PodSecurityPolicy Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Helm Notes`** (1 nodes): `Helm Installation Notes`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Helm Service`** (1 nodes): `Service Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Helm RBAC ClusterRole`** (1 nodes): `ClusterRole Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Helm ServiceAccount`** (1 nodes): `ServiceAccount Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Helm RBAC Binding`** (1 nodes): `ClusterRoleBinding Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Kustomize PSP Overlay`** (1 nodes): `PSP Calico Overlay PodSecurityPolicy`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Kustomize RBAC Base`** (1 nodes): `Kustomize Base RBAC`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `E2E Test Data`** (1 nodes): `E2E DaemonSet Test Data`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `NewServer()` connect `Mock STS/Region Clients` to `IAM & ARN Logic`, `IMDS & STS Mocks`, `CLI & Main Loop`?**
  _High betweenness centrality (0.215) - this node is a cross-community bridge._
- **Why does `NewNamespaceHandler()` connect `Namespace Handlers` to `IAM & ARN Logic`?**
  _High betweenness centrality (0.109) - this node is a cross-community bridge._
- **Are the 12 inferred relationships involving `NewServer()` (e.g. with `main()` and `newIntegServer()`) actually correct?**
  _`NewServer()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **What connects `appHandlerFunc`, `HealthResponse`, `STSClient` to the rest of the system?**
  _37 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `AWS STS Client` be split into smaller, more focused modules?**
  _Cohesion score 0.08 - nodes in this community are weakly interconnected._
- **Should `IAM & ARN Logic` be split into smaller, more focused modules?**
  _Cohesion score 0.07 - nodes in this community are weakly interconnected._
- **Should `Namespace Handlers` be split into smaller, more focused modules?**
  _Cohesion score 0.09 - nodes in this community are weakly interconnected._