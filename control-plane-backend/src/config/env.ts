import dotenv from 'dotenv';

dotenv.config();

function readNumber(value: string | undefined, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function readBoolean(value: string | undefined, fallback: boolean): boolean {
  if (value === undefined) {
    return fallback;
  }

  return ['1', 'true', 'yes', 'on'].includes(value.trim().toLowerCase());
}

const K8S_AUTH_MODES = ['auto', 'incluster', 'kubeconfig', 'token'] as const;
type K8sAuthMode = (typeof K8S_AUTH_MODES)[number];

function readK8sAuthMode(value: string | undefined): K8sAuthMode {
  if (!value) {
    return 'auto';
  }

  const normalized = value.trim().toLowerCase();
  if (K8S_AUTH_MODES.includes(normalized as K8sAuthMode)) {
    return normalized as K8sAuthMode;
  }

  return 'auto';
}

export interface Config {
  host: string;
  port: number;
  registryHost: string;
  appBaseDomain: string;
  defaultAppTtlHours: number;
  adminTokens: Set<string>;
  postgresUrl: string;
  k8sNamespace: string;
  k8sAuthMode: K8sAuthMode;
  k8sKubeconfigPath: string;
  k8sContext: string;
  k8sApiServer: string;
  k8sToken: string;
  k8sCaFile: string;
  k8sCaData: string;
  k8sSkipTlsVerify: boolean;
}

export const config: Config = {
  host: process.env.CONTROL_PLANE_HOST || '0.0.0.0',
  port: readNumber(process.env.CONTROL_PLANE_PORT, 8080),
  registryHost: process.env.REGISTRY_HOST || 'registry.internal',
  appBaseDomain: process.env.APP_BASE_DOMAIN || 'saki.internal',
  defaultAppTtlHours: readNumber(process.env.DEFAULT_APP_TTL_HOURS, 168),
  adminTokens: new Set(
    (process.env.ADMIN_TOKENS || '')
      .split(',')
      .map((token) => token.trim())
      .filter(Boolean)
  ),
  postgresUrl: process.env.POSTGRES_URL || '',
  k8sNamespace: process.env.K8S_NAMESPACE || 'saki-apps',
  k8sAuthMode: readK8sAuthMode(process.env.K8S_AUTH_MODE),
  k8sKubeconfigPath: process.env.K8S_KUBECONFIG_PATH || '',
  k8sContext: process.env.K8S_CONTEXT || '',
  k8sApiServer: process.env.K8S_API_SERVER || '',
  k8sToken: process.env.K8S_TOKEN || '',
  k8sCaFile: process.env.K8S_CA_FILE || '',
  k8sCaData: process.env.K8S_CA_DATA || '',
  k8sSkipTlsVerify: readBoolean(process.env.K8S_SKIP_TLS_VERIFY, false),
};
