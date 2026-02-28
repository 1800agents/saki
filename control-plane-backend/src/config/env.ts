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
  appIngressClassName: string;
  defaultAppTtlHours: number;
  adminTokens: Set<string>;
  postgresHost: string;
  postgresPort: number;
  postgresUser: string;
  postgresPassword: string;
  postgresDatabase: string;
  k8sNamespace: string;
  k8sKubeconfigPath: string;
}

export const config: Config = {
  host: process.env.CONTROL_PLANE_HOST || '0.0.0.0',
  port: readNumber(process.env.CONTROL_PLANE_PORT, 8080),
  registryHost: process.env.REGISTRY_HOST || 'registry.internal',
  appBaseDomain: process.env.APP_BASE_DOMAIN || 'saki.internal',
  appIngressClassName: process.env.APP_INGRESS_CLASS_NAME || '',
  defaultAppTtlHours: readNumber(process.env.DEFAULT_APP_TTL_HOURS, 168),
  adminTokens: new Set(
    (process.env.ADMIN_TOKENS || '')
      .split(',')
      .map((token) => token.trim().toLowerCase())
      .filter(Boolean)
  ),
  postgresHost: process.env.POSTGRES_HOST || 'localhost',
  postgresPort: readNumber(process.env.POSTGRES_PORT, 5432),
  postgresUser: process.env.POSTGRES_USER || 'postgres',
  postgresPassword: process.env.POSTGRES_PASSWORD || '',
  postgresDatabase: process.env.POSTGRES_DATABASE || 'saki',
  k8sNamespace: process.env.K8S_NAMESPACE || 'saki-apps',
  k8sKubeconfigPath: process.env.K8S_KUBECONFIG_PATH || '',
};
