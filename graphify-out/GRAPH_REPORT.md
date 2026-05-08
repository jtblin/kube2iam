# Graph Report - kube2iam  (2026-05-08)

## Corpus Check
- 22 files · ~22,889 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 314 nodes · 503 edges · 32 communities detected
- Extraction: 84% EXTRACTED · 16% INFERRED · 0% AMBIGUOUS · INFERRED: 82 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 19|Community 19]]
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 28|Community 28]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 30|Community 30]]
- [[_COMMUNITY_Community 31|Community 31]]

## God Nodes (most connected - your core abstractions)
1. `Server` - 15 edges
2. `NewServer()` - 14 edges
3. `newLogger()` - 14 edges
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

### Community 0 - "Community 0"
Cohesion: 0.07
Nodes (36): Client, Credentials, GetEndpointFromRegion(), getHash(), getIAMCode(), GetInstanceIAMRole(), getMetadataPath(), IsValidRegion() (+28 more)

### Community 1 - "Community 1"
Cohesion: 0.07
Nodes (22): ARN Parsing Logic, IAM Provider Interface, mockAPIError, IPTables Management, Kubernetes Client Interface, Application Entrypoint, Role Mapper, Prometheus Metrics (+14 more)

### Community 2 - "Community 2"
Cohesion: 0.17
Nodes (25): TestRunKubeconfigError(), mockRegionClient, mockSTSClient, NewServer(), buildServer(), newLogger(), newRoleMapper(), newTestIAMClient() (+17 more)

### Community 3 - "Community 3"
Cohesion: 0.13
Nodes (24): NamespaceIndexFunc(), NewNamespaceHandler(), TestIsPodActive(), TestNamespaceHandlerOnAdd(), TestNamespaceHandlerOnAddWrongType(), TestNamespaceHandlerOnDelete(), TestNamespaceHandlerOnDeleteWrongType(), TestNamespaceHandlerOnUpdate() (+16 more)

### Community 4 - "Community 4"
Cohesion: 0.1
Nodes (15): countingSTSClient, integMockIMDS, integrationIAMClient(), newIntegServer(), TestIntegErrorCaching(), TestIntegErrorCachingCurrentBehavior(), TestIntegFullRequestChain(), TestIntegHealthcheck() (+7 more)

### Community 5 - "Community 5"
Cohesion: 0.15
Nodes (8): GetNamespaceRoleAnnotation(), TestGetNamespaceRoleAnnotation(), TestGetNamespaceRoleAnnotationMissingKey(), TestGetNamespaceRoleAnnotationNilAnnotations(), NamespaceHandler, RoleMapper, RoleMappingResult, store

### Community 6 - "Community 6"
Cohesion: 0.17
Nodes (12): addFlags(), main(), GetBaseArn(), GetBaseArnWithClient(), IsValidBaseARN(), TestGetBaseArnWithClient(), TestGetBaseArnWithClientIMDSError(), TestGetBaseArnWithClientMalformedARN() (+4 more)

### Community 7 - "Community 7"
Cohesion: 0.35
Nodes (14): newNamespaceIndexer(), newPodIndexer(), newTestClient(), runningPod(), TestListNamespaces(), TestListPodIPs(), TestListPodIPsEmpty(), TestListPodIPsExcludesInactivePods() (+6 more)

### Community 8 - "Community 8"
Cohesion: 0.2
Nodes (11): TestDaemonSetScheduledOnAllNodes(), TestIPTablesRulesInstalled(), buildKubeClient(), dumpDebugInfo(), execInPod(), getDaemonSet(), kubectlApply(), loadImageIntoKind() (+3 more)

### Community 9 - "Community 9"
Cohesion: 0.14
Nodes (1): storeMock

### Community 10 - "Community 10"
Cohesion: 0.21
Nodes (2): Client, resolveDuplicatedIP()

### Community 11 - "Community 11"
Cohesion: 0.36
Nodes (5): AddRule(), checkInterfaceExists(), TestCheckInterfaceExistsFailsWithBogusInterface(), TestCheckInterfaceExistsPassesWithPlus(), TestCheckInterfaceExistsPassesWithValidInterface()

### Community 12 - "Community 12"
Cohesion: 0.7
Nodes (1): PodHandler

### Community 13 - "Community 13"
Cohesion: 0.4
Nodes (1): mockStore

### Community 14 - "Community 14"
Cohesion: 0.4
Nodes (5): Kube2iam Helm Chart, DaemonSet Template, EKS Configuration Example, Helm Chart Documentation, Helm Chart Values

### Community 15 - "Community 15"
Cohesion: 0.67
Nodes (3): Kustomize Base DaemonSet, PSP Calico Overlay DaemonSet, PSP Calico Overlay README

### Community 16 - "Community 16"
Cohesion: 1.0
Nodes (2): Kubernetes Deployment Manifest, Project Documentation

### Community 17 - "Community 17"
Cohesion: 1.0
Nodes (2): Kustomize Base Configuration, PSP Calico Kustomization

### Community 18 - "Community 18"
Cohesion: 1.0
Nodes (2): Kind Cluster E2E Config, IAM Mocks for E2E Tests

### Community 19 - "Community 19"
Cohesion: 1.0
Nodes (1): Namespace Tests

### Community 20 - "Community 20"
Cohesion: 1.0
Nodes (1): Pod Structure

### Community 21 - "Community 21"
Cohesion: 1.0
Nodes (1): Version Information

### Community 22 - "Community 22"
Cohesion: 1.0
Nodes (1): ServiceMonitor Template

### Community 23 - "Community 23"
Cohesion: 1.0
Nodes (1): PodSecurityPolicy Template

### Community 24 - "Community 24"
Cohesion: 1.0
Nodes (1): Helm Installation Notes

### Community 25 - "Community 25"
Cohesion: 1.0
Nodes (1): Service Template

### Community 26 - "Community 26"
Cohesion: 1.0
Nodes (1): ClusterRole Template

### Community 27 - "Community 27"
Cohesion: 1.0
Nodes (1): ServiceAccount Template

### Community 28 - "Community 28"
Cohesion: 1.0
Nodes (1): ClusterRoleBinding Template

### Community 29 - "Community 29"
Cohesion: 1.0
Nodes (1): PSP Calico Overlay PodSecurityPolicy

### Community 30 - "Community 30"
Cohesion: 1.0
Nodes (1): Kustomize Base RBAC

### Community 31 - "Community 31"
Cohesion: 1.0
Nodes (1): E2E DaemonSet Test Data

## Knowledge Gaps
- **37 isolated node(s):** `appHandlerFunc`, `HealthResponse`, `STSClient`, `RegionClient`, `IMDSClient` (+32 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 9`** (14 nodes): `mapper_test.go`, `TestCheckRoleForNamespace()`, `TestDumpDebugInfo()`, `TestExtractRoleARN()`, `TestGetExternalIDMappingWithAnnotation()`, `TestGetExternalIDMappingWithoutAnnotation()`, `TestGetRoleMappingNoAnnotationNoDefault()`, `TestGetRoleMappingPodNotFound()`, `TestGetRoleMappingWithDefault()`, `storeMock`, `.ListNamespaces()`, `.ListPodIPs()`, `.NamespaceByName()`, `.PodByIP()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 10`** (12 nodes): `Client`, `.createNamespaceLW()`, `.createPodLW()`, `.ListNamespaces()`, `.ListPodIPs()`, `.NamespaceByName()`, `.PodByIP()`, `.WatchForNamespaces()`, `.WatchForPods()`, `k8s.go`, `NewClient()`, `resolveDuplicatedIP()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 12`** (5 nodes): `PodHandler`, `.OnAdd()`, `.OnDelete()`, `.OnUpdate()`, `.podFields()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 13`** (5 nodes): `mockStore`, `.ListNamespaces()`, `.ListPodIPs()`, `.NamespaceByName()`, `.PodByIP()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 16`** (2 nodes): `Kubernetes Deployment Manifest`, `Project Documentation`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 17`** (2 nodes): `Kustomize Base Configuration`, `PSP Calico Kustomization`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 18`** (2 nodes): `Kind Cluster E2E Config`, `IAM Mocks for E2E Tests`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 19`** (1 nodes): `Namespace Tests`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 20`** (1 nodes): `Pod Structure`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 21`** (1 nodes): `Version Information`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 22`** (1 nodes): `ServiceMonitor Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 23`** (1 nodes): `PodSecurityPolicy Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 24`** (1 nodes): `Helm Installation Notes`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 25`** (1 nodes): `Service Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 26`** (1 nodes): `ClusterRole Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 27`** (1 nodes): `ServiceAccount Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 28`** (1 nodes): `ClusterRoleBinding Template`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 29`** (1 nodes): `PSP Calico Overlay PodSecurityPolicy`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 30`** (1 nodes): `Kustomize Base RBAC`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Community 31`** (1 nodes): `E2E DaemonSet Test Data`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `NewServer()` connect `Community 2` to `Community 1`, `Community 4`, `Community 6`?**
  _High betweenness centrality (0.219) - this node is a cross-community bridge._
- **Why does `main()` connect `Community 6` to `Community 0`, `Community 2`, `Community 11`?**
  _High betweenness centrality (0.108) - this node is a cross-community bridge._
- **Are the 13 inferred relationships involving `NewServer()` (e.g. with `main()` and `newIntegServer()`) actually correct?**
  _`NewServer()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **What connects `appHandlerFunc`, `HealthResponse`, `STSClient` to the rest of the system?**
  _37 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.07 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.07 - nodes in this community are weakly interconnected._
- **Should `Community 3` be split into smaller, more focused modules?**
  _Cohesion score 0.13 - nodes in this community are weakly interconnected._