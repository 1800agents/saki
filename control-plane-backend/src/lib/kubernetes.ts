import * as fs from 'node:fs';
import * as k8s from '@kubernetes/client-node';
import { config } from '../config/env';
import type { AppRecord, AppStatus, LogsPage } from '../types/app';

type KubeAuthSource = 'incluster' | 'kubeconfig';

interface KubeConfigResolution {
  kubeConfig: k8s.KubeConfig;
  source: KubeAuthSource;
}

interface KubeBootstrapDiagnostics {
  kubernetesServiceHost: string;
  inClusterTokenPath: string;
  inClusterTokenExists: boolean;
  inClusterCredentialsDetected: boolean;
  kubeconfigPath: string;
  kubeconfigPathExists: boolean;
}

interface FindByOwnerAndNameInput {
  owner: string;
  name: string;
}

interface ListAppsInput {
  owner?: string;
}

interface UpsertAppResourcesInput {
  app: AppRecord;
  databaseUrl: string;
  replicas: number;
}

interface SetAppReplicasInput {
  app: AppRecord;
  replicas: number;
  status: AppStatus;
}

const IN_CLUSTER_TOKEN_PATH = '/var/run/secrets/kubernetes.io/serviceaccount/token';
const CONTROL_PLANE_LABEL_VALUE = 'saki-control-plane';

const LABEL_MANAGED_BY = 'app.kubernetes.io/managed-by';
const LABEL_OWNER = 'saki.dev/owner';
const LABEL_APP_NAME = 'saki.dev/name';
const LABEL_APP_ID = 'saki.dev/app-id';

const ANN_APP_ID = 'saki.dev/app-id';
const ANN_DEPLOYMENT_ID = 'saki.dev/deployment-id';
const ANN_OWNER = 'saki.dev/owner';
const ANN_NAME = 'saki.dev/name';
const ANN_DESCRIPTION = 'saki.dev/description';
const ANN_URL = 'saki.dev/url';
const ANN_IMAGE = 'saki.dev/image';
const ANN_CREATED_AT = 'saki.dev/created-at';
const ANN_UPDATED_AT = 'saki.dev/updated-at';
const ANN_TTL_EXPIRY = 'saki.dev/ttl-expiry';
const ANN_STATUS = 'saki.dev/status';

const APP_CONTAINER_NAME = 'app';
const APP_PORT = 3000;
const SERVICE_PORT = 80;

function hasInClusterCredentials(): boolean {
  return Boolean(process.env.KUBERNETES_SERVICE_HOST) && fs.existsSync(IN_CLUSTER_TOKEN_PATH);
}

function getKubeBootstrapDiagnostics(): KubeBootstrapDiagnostics {
  const kubeconfigPath = config.k8sKubeconfigPath || '(unset)';
  return {
    kubernetesServiceHost: process.env.KUBERNETES_SERVICE_HOST || '(unset)',
    inClusterTokenPath: IN_CLUSTER_TOKEN_PATH,
    inClusterTokenExists: fs.existsSync(IN_CLUSTER_TOKEN_PATH),
    inClusterCredentialsDetected: hasInClusterCredentials(),
    kubeconfigPath,
    kubeconfigPathExists: kubeconfigPath !== '(unset)' && fs.existsSync(kubeconfigPath),
  };
}

function buildFailureHint(reason: string, diagnostics: KubeBootstrapDiagnostics): string {
  if (!diagnostics.inClusterCredentialsDetected && diagnostics.kubeconfigPath === '(unset)') {
    return 'Set K8S_KUBECONFIG_PATH when running outside Kubernetes.';
  }

  if (!diagnostics.inClusterCredentialsDetected && !diagnostics.kubeconfigPathExists) {
    return `Kubeconfig file not found at ${diagnostics.kubeconfigPath}.`;
  }

  if (reason.includes('No active cluster')) {
    return `Check kubeconfig current-context in ${diagnostics.kubeconfigPath}.`;
  }

  return 'Verify in-cluster service account access or kubeconfig current-context/cluster values.';
}

function loadFromKubeconfigPath(path: string): k8s.KubeConfig {
  if (!fs.existsSync(path)) {
    throw new Error(`Kubeconfig file does not exist: ${path}`);
  }

  const kubeConfig = new k8s.KubeConfig();
  kubeConfig.loadFromFile(path);

  const currentContext = kubeConfig.getCurrentContext();
  if (!currentContext) {
    const contexts = kubeConfig
      .getContexts()
      .map((ctx) => ctx.name)
      .join(', ');
    throw new Error(
      `Kubeconfig file has no current-context set: ${path}${contexts ? ` (available contexts: ${contexts})` : ''}`
    );
  }

  if (!kubeConfig.getCurrentCluster()) {
    throw new Error(
      `Kubeconfig current-context "${currentContext}" does not resolve to an active cluster: ${path}`
    );
  }

  return kubeConfig;
}

function resolveKubeConfig(): KubeConfigResolution {
  if (hasInClusterCredentials()) {
    const kubeConfig = new k8s.KubeConfig();
    kubeConfig.loadFromCluster();
    return { kubeConfig, source: 'incluster' };
  }

  if (!config.k8sKubeconfigPath) {
    throw new Error(
      'K8S_KUBECONFIG_PATH is required when running outside Kubernetes (in-cluster credentials not detected)'
    );
  }

  return {
    kubeConfig: loadFromKubeconfigPath(config.k8sKubeconfigPath),
    source: 'kubeconfig',
  };
}

function normalizeOwner(owner: string): string {
  return owner.toLowerCase();
}

function toKubeNameFragment(value: string, maxLength: number): string {
  const normalized = value
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-+/, '')
    .replace(/-+$/, '');
  const sliced = normalized.slice(0, maxLength).replace(/-+$/, '');
  return sliced || 'app';
}

function toLabelValue(value: string, fallback = 'x'): string {
  const normalized = value
    .toLowerCase()
    .replace(/[^a-z0-9._-]/g, '-')
    .replace(/^-+/, '')
    .replace(/-+$/, '')
    .slice(0, 63);

  if (!normalized) {
    return fallback;
  }

  const startsOk = /[a-z0-9]/.test(normalized[0]);
  const endsOk = /[a-z0-9]/.test(normalized[normalized.length - 1]);
  if (startsOk && endsOk) {
    return normalized;
  }

  return normalized.replace(/^[^a-z0-9]+/, '').replace(/[^a-z0-9]+$/, '') || fallback;
}

function appResourceBaseName(app: AppRecord): string {
  const namePart = toKubeNameFragment(app.name, 32);
  const idPart = app.app_id.toLowerCase().replace(/[^a-z0-9]/g, '').slice(0, 10) || 'app';
  return `saki-${namePart}-${idPart}`;
}

function appResourceNames(app: AppRecord): { deployment: string; service: string; ingress: string } {
  const base = appResourceBaseName(app);
  return {
    deployment: `${base}-dep`,
    service: `${base}-svc`,
    ingress: `${base}-ing`,
  };
}

function safeHostForAppUrl(url: string, name: string): string {
  try {
    return new URL(url).hostname;
  } catch {
    return `${name}.${config.appBaseDomain}`;
  }
}

function buildSelector(parts: Record<string, string>): string {
  return Object.entries(parts)
    .map(([key, value]) => `${key}=${value}`)
    .join(',');
}

function decodeCursor(cursor?: string): number {
  if (!cursor) {
    return 0;
  }

  try {
    const raw = Buffer.from(cursor, 'base64url').toString('utf8');
    const offset = Number(raw);
    return Number.isFinite(offset) && offset >= 0 ? Math.trunc(offset) : 0;
  } catch {
    return 0;
  }
}

function encodeCursor(offset: number): string {
  return Buffer.from(String(offset), 'utf8').toString('base64url');
}

function parseLogLine(line: string): { timestamp: string; stream: 'stdout' | 'stderr'; message: string } {
  const trimmed = line.trimEnd();
  const firstSpace = trimmed.indexOf(' ');
  if (firstSpace > 0) {
    const maybeTimestamp = trimmed.slice(0, firstSpace);
    if (!Number.isNaN(Date.parse(maybeTimestamp))) {
      return {
        timestamp: maybeTimestamp,
        stream: 'stdout',
        message: trimmed.slice(firstSpace + 1),
      };
    }
  }

  return {
    timestamp: new Date().toISOString(),
    stream: 'stdout',
    message: trimmed,
  };
}

function statusFromDeployment(deployment: k8s.V1Deployment): AppStatus {
  if (deployment.metadata?.deletionTimestamp) {
    return 'deleting';
  }

  const desiredReplicas = deployment.spec?.replicas ?? 1;
  if (desiredReplicas === 0) {
    return 'stopped';
  }

  const conditions = deployment.status?.conditions || [];
  const progressing = conditions.find((condition) => condition.type === 'Progressing');
  if (progressing?.status === 'False' && progressing.reason === 'ProgressDeadlineExceeded') {
    return 'failed';
  }

  const ready = deployment.status?.readyReplicas ?? 0;
  const available = deployment.status?.availableReplicas ?? 0;
  const observedGeneration = deployment.status?.observedGeneration ?? 0;
  const generation = deployment.metadata?.generation ?? 0;

  if (ready > 0 && available > 0 && observedGeneration >= generation) {
    return 'healthy';
  }

  if ((deployment.status?.replicas ?? 0) === 0) {
    return 'pending';
  }

  return 'deploying';
}

function labelsForApp(app: AppRecord): Record<string, string> {
  return {
    [LABEL_MANAGED_BY]: CONTROL_PLANE_LABEL_VALUE,
    [LABEL_OWNER]: toLabelValue(normalizeOwner(app.owner)),
    [LABEL_APP_NAME]: toLabelValue(app.name),
    [LABEL_APP_ID]: toLabelValue(app.app_id),
  };
}

function selectorLabelsForApp(app: AppRecord): Record<string, string> {
  return {
    [LABEL_MANAGED_BY]: CONTROL_PLANE_LABEL_VALUE,
    [LABEL_APP_ID]: toLabelValue(app.app_id),
  };
}

function annotationsForApp(app: AppRecord, status: AppStatus): Record<string, string> {
  return {
    [ANN_APP_ID]: app.app_id,
    [ANN_DEPLOYMENT_ID]: app.deployment_id,
    [ANN_OWNER]: normalizeOwner(app.owner),
    [ANN_NAME]: app.name,
    [ANN_DESCRIPTION]: app.description,
    [ANN_URL]: app.url,
    [ANN_IMAGE]: app.image,
    [ANN_CREATED_AT]: app.created_at,
    [ANN_UPDATED_AT]: app.updated_at,
    [ANN_TTL_EXPIRY]: app.ttl_expiry,
    [ANN_STATUS]: status,
  };
}

function toIsoString(value: string | Date | undefined): string | undefined {
  if (!value) {
    return undefined;
  }

  if (typeof value === 'string') {
    return value;
  }

  return value.toISOString();
}

function deploymentToRecord(deployment: k8s.V1Deployment): AppRecord | null {
  const annotations = deployment.metadata?.annotations || {};
  const appId = annotations[ANN_APP_ID];
  const deploymentId = annotations[ANN_DEPLOYMENT_ID];
  const owner = annotations[ANN_OWNER];
  const name = annotations[ANN_NAME];
  const description = annotations[ANN_DESCRIPTION];
  const url = annotations[ANN_URL];
  const createdAt = annotations[ANN_CREATED_AT];
  const updatedAt = annotations[ANN_UPDATED_AT];
  const ttlExpiry = annotations[ANN_TTL_EXPIRY];
  const image = annotations[ANN_IMAGE] || deployment.spec?.template?.spec?.containers?.[0]?.image;

  if (!appId || !deploymentId || !owner || !name || !description || !url || !image) {
    return null;
  }

  const createdTimestamp = toIsoString(deployment.metadata?.creationTimestamp);

  return {
    app_id: appId,
    deployment_id: deploymentId,
    owner,
    name,
    description,
    image,
    url,
    status: statusFromDeployment(deployment),
    created_at: createdAt || createdTimestamp || new Date().toISOString(),
    updated_at: updatedAt || createdTimestamp || new Date().toISOString(),
    ttl_expiry: ttlExpiry || new Date().toISOString(),
  };
}

function isNotFoundError(error: unknown): boolean {
  return error instanceof k8s.ApiException && error.code === 404;
}

export class KubernetesDeployer {
  private readonly namespace: string;
  private readonly enabled: boolean;
  private readonly authSource?: KubeAuthSource;
  private readonly initError?: unknown;

  private readonly appsApi?: k8s.AppsV1Api;
  private readonly coreApi?: k8s.CoreV1Api;
  private readonly networkingApi?: k8s.NetworkingV1Api;

  constructor() {
    this.namespace = config.k8sNamespace;

    try {
      const resolved = resolveKubeConfig();
      this.authSource = resolved.source;
      this.appsApi = resolved.kubeConfig.makeApiClient(k8s.AppsV1Api);
      this.coreApi = resolved.kubeConfig.makeApiClient(k8s.CoreV1Api);
      this.networkingApi = resolved.kubeConfig.makeApiClient(k8s.NetworkingV1Api);
      this.enabled = true;
      console.log(`Kubernetes client initialized (auth=${this.authSource}, namespace=${this.namespace})`);
    } catch (error) {
      this.enabled = false;
      this.initError = error;
      const reason = error instanceof Error ? error.message : String(error);
      const diagnostics = getKubeBootstrapDiagnostics();
      const hint = buildFailureHint(reason, diagnostics);
      console.warn(
        `Kubernetes client bootstrap failed (${reason}). ` +
          `Diagnostics: service_host=${diagnostics.kubernetesServiceHost}, ` +
          `token_path=${diagnostics.inClusterTokenPath}, token_exists=${diagnostics.inClusterTokenExists}, ` +
          `incluster_detected=${diagnostics.inClusterCredentialsDetected}, ` +
          `kubeconfig_path=${diagnostics.kubeconfigPath}, kubeconfig_exists=${diagnostics.kubeconfigPathExists}. ` +
          `Hint: ${hint}`
      );
    }
  }

  private ensureEnabled(): void {
    if (this.enabled) {
      return;
    }

    const reason = this.initError instanceof Error ? this.initError.message : String(this.initError);
    throw new Error(`Kubernetes client is not available: ${reason}`);
  }

  private getAppsApi(): k8s.AppsV1Api {
    this.ensureEnabled();
    if (!this.appsApi) {
      throw new Error('AppsV1Api client is unavailable');
    }
    return this.appsApi;
  }

  private getCoreApi(): k8s.CoreV1Api {
    this.ensureEnabled();
    if (!this.coreApi) {
      throw new Error('CoreV1Api client is unavailable');
    }
    return this.coreApi;
  }

  private getNetworkingApi(): k8s.NetworkingV1Api {
    this.ensureEnabled();
    if (!this.networkingApi) {
      throw new Error('NetworkingV1Api client is unavailable');
    }
    return this.networkingApi;
  }

  async findAppByOwnerAndName({ owner, name }: FindByOwnerAndNameInput): Promise<AppRecord | null> {
    const appsApi = this.getAppsApi();
    const selector = buildSelector({
      [LABEL_MANAGED_BY]: CONTROL_PLANE_LABEL_VALUE,
      [LABEL_OWNER]: toLabelValue(normalizeOwner(owner)),
      [LABEL_APP_NAME]: toLabelValue(name),
    });

    const deployments = await appsApi.listNamespacedDeployment({
      namespace: this.namespace,
      labelSelector: selector,
    });

    const matches = (deployments.items || []).map(deploymentToRecord).filter(Boolean) as AppRecord[];
    if (matches.length > 1) {
      throw new Error(`Multiple deployments found for owner=${owner} name=${name}`);
    }

    return matches[0] || null;
  }

  async findAppById(appId: string): Promise<AppRecord | null> {
    const appsApi = this.getAppsApi();
    const selector = buildSelector({
      [LABEL_MANAGED_BY]: CONTROL_PLANE_LABEL_VALUE,
      [LABEL_APP_ID]: toLabelValue(appId),
    });

    const deployments = await appsApi.listNamespacedDeployment({
      namespace: this.namespace,
      labelSelector: selector,
    });

    const matches = (deployments.items || []).map(deploymentToRecord).filter(Boolean) as AppRecord[];
    if (matches.length > 1) {
      throw new Error(`Multiple deployments found for app_id=${appId}`);
    }

    return matches[0] || null;
  }

  async listApps(input: ListAppsInput = {}): Promise<AppRecord[]> {
    const appsApi = this.getAppsApi();
    const selectors: Record<string, string> = {
      [LABEL_MANAGED_BY]: CONTROL_PLANE_LABEL_VALUE,
    };

    if (input.owner) {
      selectors[LABEL_OWNER] = toLabelValue(normalizeOwner(input.owner));
    }

    const deployments = await appsApi.listNamespacedDeployment({
      namespace: this.namespace,
      labelSelector: buildSelector(selectors),
    });

    const records = (deployments.items || []).map(deploymentToRecord).filter((record): record is AppRecord => {
      return record !== null;
    });

    return records.sort((a, b) => b.updated_at.localeCompare(a.updated_at));
  }

  async upsertAppResources({ app, databaseUrl, replicas }: UpsertAppResourcesInput): Promise<void> {
    const appsApi = this.getAppsApi();
    const coreApi = this.getCoreApi();
    const networkingApi = this.getNetworkingApi();

    const labels = labelsForApp(app);
    const selectorLabels = selectorLabelsForApp(app);
    const annotations = annotationsForApp(app, 'deploying');
    const resources = appResourceNames(app);
    const host = safeHostForAppUrl(app.url, app.name);

    const deploymentBody: k8s.V1Deployment = {
      apiVersion: 'apps/v1',
      kind: 'Deployment',
      metadata: {
        name: resources.deployment,
        namespace: this.namespace,
        labels,
        annotations,
      },
      spec: {
        replicas,
        selector: { matchLabels: selectorLabels },
        template: {
          metadata: {
            labels: {
              ...selectorLabels,
              [LABEL_OWNER]: labels[LABEL_OWNER],
              [LABEL_APP_NAME]: labels[LABEL_APP_NAME],
            },
          },
          spec: {
            containers: [
              {
                name: APP_CONTAINER_NAME,
                image: app.image,
                imagePullPolicy: 'Always',
                ports: [{ name: 'http', containerPort: APP_PORT }],
                env: [
                  { name: 'PORT', value: String(APP_PORT) },
                  { name: 'DATABASE_URL', value: databaseUrl },
                ],
              },
            ],
          },
        },
      },
    };

    const serviceBody: k8s.V1Service = {
      apiVersion: 'v1',
      kind: 'Service',
      metadata: {
        name: resources.service,
        namespace: this.namespace,
        labels,
        annotations,
      },
      spec: {
        selector: selectorLabels,
        ports: [
          {
            name: 'http',
            port: SERVICE_PORT,
            targetPort: APP_PORT,
            protocol: 'TCP',
          },
        ],
      },
    };

    const ingressBody: k8s.V1Ingress = {
      apiVersion: 'networking.k8s.io/v1',
      kind: 'Ingress',
      metadata: {
        name: resources.ingress,
        namespace: this.namespace,
        labels,
        annotations,
      },
      spec: {
        ingressClassName: config.appIngressClassName || undefined,
        rules: [
          {
            host,
            http: {
              paths: [
                {
                  path: '/',
                  pathType: 'Prefix',
                  backend: {
                    service: {
                      name: resources.service,
                      port: { number: SERVICE_PORT },
                    },
                  },
                },
              ],
            },
          },
        ],
      },
    };

    await this.createOrReplaceDeployment(appsApi, deploymentBody);
    await this.createOrReplaceService(coreApi, serviceBody);
    await this.createOrReplaceIngress(networkingApi, ingressBody);
  }

  async setAppReplicas({ app, replicas, status }: SetAppReplicasInput): Promise<AppRecord> {
    const appsApi = this.getAppsApi();
    const resources = appResourceNames(app);
    const deployment = await appsApi.readNamespacedDeployment({
      name: resources.deployment,
      namespace: this.namespace,
    });

    const annotations = {
      ...(deployment.metadata?.annotations || {}),
      [ANN_UPDATED_AT]: new Date().toISOString(),
      [ANN_STATUS]: status,
    };

    if (!deployment.spec) {
      throw new Error(`Deployment ${resources.deployment} has no spec`);
    }

    const nextDeployment: k8s.V1Deployment = {
      ...deployment,
      metadata: {
        ...deployment.metadata,
        annotations,
      },
      spec: {
        ...deployment.spec,
        replicas,
      },
    };

    await appsApi.replaceNamespacedDeployment({
      name: resources.deployment,
      namespace: this.namespace,
      body: nextDeployment,
    });

    const updated = await this.findAppById(app.app_id);
    if (!updated) {
      throw new Error(`Deployment disappeared after replica change for app_id=${app.app_id}`);
    }

    return updated;
  }

  async deleteAppResources(app: AppRecord): Promise<void> {
    const appsApi = this.getAppsApi();
    const coreApi = this.getCoreApi();
    const networkingApi = this.getNetworkingApi();
    const resources = appResourceNames(app);

    await Promise.all([
      (async () => {
        try {
          await networkingApi.deleteNamespacedIngress({
            name: resources.ingress,
            namespace: this.namespace,
          });
        } catch (error) {
          if (!isNotFoundError(error)) {
            throw error;
          }
        }
      })(),
      (async () => {
        try {
          await coreApi.deleteNamespacedService({
            name: resources.service,
            namespace: this.namespace,
          });
        } catch (error) {
          if (!isNotFoundError(error)) {
            throw error;
          }
        }
      })(),
      (async () => {
        try {
          await appsApi.deleteNamespacedDeployment({
            name: resources.deployment,
            namespace: this.namespace,
          });
        } catch (error) {
          if (!isNotFoundError(error)) {
            throw error;
          }
        }
      })(),
    ]);
  }

  async readLogs(app: AppRecord, cursor?: string, limit = 200): Promise<LogsPage> {
    const coreApi = this.getCoreApi();
    const pods = await coreApi.listNamespacedPod({
      namespace: this.namespace,
      labelSelector: buildSelector({
        [LABEL_MANAGED_BY]: CONTROL_PLANE_LABEL_VALUE,
        [LABEL_APP_ID]: toLabelValue(app.app_id),
      }),
    });

    const items = pods.items || [];
    if (items.length === 0) {
      return { data: [], next_cursor: null };
    }

    const runningPod =
      items.find((pod) => pod.status?.phase === 'Running') ||
      items.sort((a, b) => {
        const aTime = a.status?.startTime ? new Date(a.status.startTime).getTime() : 0;
        const bTime = b.status?.startTime ? new Date(b.status.startTime).getTime() : 0;
        return bTime - aTime;
      })[0];

    const podName = runningPod.metadata?.name;
    if (!podName) {
      return { data: [], next_cursor: null };
    }

    const container = runningPod.spec?.containers?.[0]?.name;
    let rawLogs = '';
    try {
      rawLogs = await coreApi.readNamespacedPodLog({
        name: podName,
        namespace: this.namespace,
        container,
        tailLines: 5000,
        timestamps: true,
      });
    } catch (error) {
      if (!(error instanceof k8s.ApiException) || error.code !== 400) {
        throw error;
      }

      rawLogs = await coreApi.readNamespacedPodLog({
        name: podName,
        namespace: this.namespace,
        tailLines: 5000,
        timestamps: true,
      });
    }

    const lines = rawLogs
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);
    const offset = decodeCursor(cursor);
    const page = lines.slice(offset, offset + limit).map(parseLogLine);
    const nextOffset = offset + page.length;

    return {
      data: page,
      next_cursor: nextOffset < lines.length ? encodeCursor(nextOffset) : null,
    };
  }

  private async createOrReplaceDeployment(api: k8s.AppsV1Api, body: k8s.V1Deployment): Promise<void> {
    const name = body.metadata?.name;
    if (!name) {
      throw new Error('Deployment metadata.name is required');
    }

    try {
      const existing = await api.readNamespacedDeployment({
        name,
        namespace: this.namespace,
      });

      await api.replaceNamespacedDeployment({
        name,
        namespace: this.namespace,
        body: {
          ...body,
          metadata: {
            ...body.metadata,
            resourceVersion: existing.metadata?.resourceVersion,
          },
        },
      });
    } catch (error) {
      if (!isNotFoundError(error)) {
        throw error;
      }

      await api.createNamespacedDeployment({
        namespace: this.namespace,
        body,
      });
    }
  }

  private async createOrReplaceService(api: k8s.CoreV1Api, body: k8s.V1Service): Promise<void> {
    const name = body.metadata?.name;
    if (!name) {
      throw new Error('Service metadata.name is required');
    }

    try {
      const existing = await api.readNamespacedService({
        name,
        namespace: this.namespace,
      });

      await api.replaceNamespacedService({
        name,
        namespace: this.namespace,
        body: {
          ...body,
          metadata: {
            ...body.metadata,
            resourceVersion: existing.metadata?.resourceVersion,
          },
        },
      });
    } catch (error) {
      if (!isNotFoundError(error)) {
        throw error;
      }

      await api.createNamespacedService({
        namespace: this.namespace,
        body,
      });
    }
  }

  private async createOrReplaceIngress(api: k8s.NetworkingV1Api, body: k8s.V1Ingress): Promise<void> {
    const name = body.metadata?.name;
    if (!name) {
      throw new Error('Ingress metadata.name is required');
    }

    try {
      const existing = await api.readNamespacedIngress({
        name,
        namespace: this.namespace,
      });

      await api.replaceNamespacedIngress({
        name,
        namespace: this.namespace,
        body: {
          ...body,
          metadata: {
            ...body.metadata,
            resourceVersion: existing.metadata?.resourceVersion,
          },
        },
      });
    } catch (error) {
      if (!isNotFoundError(error)) {
        throw error;
      }

      await api.createNamespacedIngress({
        namespace: this.namespace,
        body,
      });
    }
  }
}
