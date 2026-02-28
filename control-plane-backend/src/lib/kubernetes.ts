import * as fs from 'node:fs';
import * as k8s from '@kubernetes/client-node';
import { config } from '../config/env';
import type { AppRecord, LogsPage } from '../types/app';

type KubeAuthSource = 'incluster' | 'kubeconfig';

interface KubeConfigResolution {
  kubeConfig: k8s.KubeConfig;
  source: KubeAuthSource;
}

const IN_CLUSTER_TOKEN_PATH = '/var/run/secrets/kubernetes.io/serviceaccount/token';

function hasInClusterCredentials(): boolean {
  return Boolean(process.env.KUBERNETES_SERVICE_HOST) && fs.existsSync(IN_CLUSTER_TOKEN_PATH);
}

function loadFromKubeconfigPath(path: string): k8s.KubeConfig {
  if (!fs.existsSync(path)) {
    throw new Error(`Kubeconfig file does not exist: ${path}`);
  }

  const kubeConfig = new k8s.KubeConfig();
  kubeConfig.loadFromFile(path);
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

export class KubernetesDeployer {
  private readonly namespace: string;
  private readonly enabled: boolean;
  private readonly authSource?: KubeAuthSource;
  private readonly initError?: unknown;

  private readonly kubeConfig?: k8s.KubeConfig;
  private readonly appsApi?: k8s.AppsV1Api;
  private readonly coreApi?: k8s.CoreV1Api;
  private readonly networkingApi?: k8s.NetworkingV1Api;
  private readonly batchApi?: k8s.BatchV1Api;
  private readonly objectApi?: k8s.KubernetesObjectApi;

  constructor() {
    this.namespace = config.k8sNamespace;

    try {
      const resolved = resolveKubeConfig();
      this.kubeConfig = resolved.kubeConfig;
      this.authSource = resolved.source;
      this.appsApi = resolved.kubeConfig.makeApiClient(k8s.AppsV1Api);
      this.coreApi = resolved.kubeConfig.makeApiClient(k8s.CoreV1Api);
      this.networkingApi = resolved.kubeConfig.makeApiClient(k8s.NetworkingV1Api);
      this.batchApi = resolved.kubeConfig.makeApiClient(k8s.BatchV1Api);
      this.objectApi = k8s.KubernetesObjectApi.makeApiClient(resolved.kubeConfig);
      this.enabled = true;
      console.log(`Kubernetes client initialized (auth=${this.authSource}, namespace=${this.namespace})`);
    } catch (error) {
      this.enabled = false;
      this.initError = error;
      const reason = error instanceof Error ? error.message : String(error);
      console.warn(`Kubernetes client bootstrap failed (${reason}). Running in no-op mode.`);
    }
  }

  async deployApp(app: AppRecord): Promise<void> {
    if (!this.enabled) {
      return;
    }

    console.log(
      `deployApp: ${app.app_id} -> ${app.image} in namespace ${this.namespace} (auth=${this.authSource})`
    );
  }

  async stopApp(app: AppRecord): Promise<void> {
    if (!this.enabled) {
      return;
    }

    console.log(`stopApp: ${app.app_id} in namespace ${this.namespace}`);
  }

  async startApp(app: AppRecord): Promise<void> {
    if (!this.enabled) {
      return;
    }

    console.log(`startApp: ${app.app_id} in namespace ${this.namespace}`);
  }

  async deleteApp(app: AppRecord): Promise<void> {
    if (!this.enabled) {
      return;
    }

    console.log(`deleteApp: ${app.app_id} in namespace ${this.namespace}`);
  }

  async readLogs(_app: AppRecord, _cursor?: string, _limit?: number): Promise<LogsPage> {
    if (!this.enabled) {
      return { data: [], next_cursor: null };
    }

    return { data: [], next_cursor: null };
  }
}
