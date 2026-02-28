import dotenv from 'dotenv';

dotenv.config();

function readNumber(value: string | undefined, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
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
  k8sKubeconfigPath: string;
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
  k8sKubeconfigPath: process.env.K8S_KUBECONFIG_PATH || '',
};
