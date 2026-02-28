import { randomUUID } from 'node:crypto';
import { ApiError } from '../middleware/error-handler';
import { KubernetesDeployer } from '../lib/kubernetes';
import { PostgresProvisioner } from '../lib/postgres';
import { config } from '../config/env';
import type { AppDetail, AppRecord, AppSummary, LogsPage } from '../types/app';

interface AppServiceDeps {
  kubernetesDeployer: KubernetesDeployer;
  postgresProvisioner: PostgresProvisioner;
}

interface PreparePushInput {
  owner: string;
  name: string;
  gitCommit: string;
}

interface UpsertAppInput {
  owner: string;
  name: string;
  description: string;
  image: string;
}

interface AppIdInput {
  owner: string;
  appId: string;
}

interface ListAppsInput {
  owner: string;
  includeAll: boolean;
  isAdmin: boolean;
}

interface GetLogsInput extends AppIdInput {
  cursor?: string;
  limit: number;
}

interface PreparePushResponse {
  repository: string;
  push_token: string;
  expires_at: string;
  required_tag: string;
}

interface UpsertAppResponse {
  app_id: string;
  deployment_id: string;
  url: string;
  status: AppRecord['status'];
}

interface StatusResponse {
  app_id: string;
  status: AppRecord['status'];
}

interface ListAppsResponse {
  data: AppSummary[];
}

function computeTtlExpiry(): string {
  const expiry = new Date(Date.now() + config.defaultAppTtlHours * 60 * 60 * 1000);
  return expiry.toISOString();
}

function toAppSummary(app: AppRecord): AppSummary {
  return {
    app_id: app.app_id,
    name: app.name,
    status: app.status,
    url: app.url,
  };
}

function toAppDetail(app: AppRecord): AppDetail {
  return {
    app_id: app.app_id,
    deployment_id: app.deployment_id,
    name: app.name,
    description: app.description,
    url: app.url,
    status: app.status,
    created_at: app.created_at,
    updated_at: app.updated_at,
    image: app.image,
    ttl_expiry: app.ttl_expiry,
    owner: app.owner,
  };
}

export class AppService {
  private readonly kubernetesDeployer: KubernetesDeployer;
  private readonly postgresProvisioner: PostgresProvisioner;

  constructor({ kubernetesDeployer, postgresProvisioner }: AppServiceDeps) {
    this.kubernetesDeployer = kubernetesDeployer;
    this.postgresProvisioner = postgresProvisioner;
  }

  preparePush({ owner, name, gitCommit }: PreparePushInput): PreparePushResponse {
    const requiredTag = gitCommit.slice(0, 7).toLowerCase();

    return {
      repository: `${config.registryHost}/${owner}/${name}`,
      push_token: randomUUID(),
      expires_at: new Date(Date.now() + 10 * 60 * 1000).toISOString(),
      required_tag: requiredTag,
    };
  }

  async upsertApp({ owner, name, description, image }: UpsertAppInput): Promise<UpsertAppResponse> {
    const expectedRepository = `${config.registryHost}/${owner}/${name}`;

    if (!image.startsWith(`${expectedRepository}:`)) {
      throw new ApiError(400, 'invalid_image_namespace', 'Image must match owner/app namespace', {
        expected_prefix: `${expectedRepository}:<tag>`,
      });
    }

    const url = `https://${name}.${config.appBaseDomain}`;
    const now = new Date().toISOString();
    const ttlExpiry = computeTtlExpiry();
    const existing = await this.kubernetesDeployer.findAppByOwnerAndName({ owner, name });

    const app: AppRecord = {
      app_id: existing?.app_id || randomUUID(),
      deployment_id: existing?.deployment_id || randomUUID(),
      owner,
      name,
      description,
      image,
      url,
      status: 'deploying',
      created_at: existing?.created_at || now,
      updated_at: now,
      ttl_expiry: ttlExpiry,
    };

    await this.postgresProvisioner.ensureSchema(app.app_id);
    const databaseUrl = this.postgresProvisioner.databaseUrlForApp(app.app_id);
    await this.kubernetesDeployer.upsertAppResources({
      app,
      databaseUrl,
      replicas: 1,
    });

    return {
      app_id: app.app_id,
      deployment_id: app.deployment_id,
      url: app.url,
      status: app.status,
    };
  }

  async getApp({ owner, appId }: AppIdInput): Promise<AppDetail> {
    const app = await this.kubernetesDeployer.findAppById(appId);

    if (!app || app.owner !== owner) {
      throw new ApiError(404, 'not_found', 'App not found');
    }

    return toAppDetail(app);
  }

  async listApps({ owner, includeAll, isAdmin }: ListAppsInput): Promise<ListAppsResponse> {
    if (includeAll && !isAdmin) {
      throw new ApiError(403, 'forbidden', 'Listing all apps requires admin access');
    }

    const apps = includeAll
      ? await this.kubernetesDeployer.listApps()
      : await this.kubernetesDeployer.listApps({ owner });

    return {
      data: apps.map(toAppSummary),
    };
  }

  async stopApp({ owner, appId }: AppIdInput): Promise<StatusResponse> {
    const app = await this.kubernetesDeployer.findAppById(appId);

    if (!app || app.owner !== owner) {
      throw new ApiError(404, 'not_found', 'App not found');
    }

    const updated = await this.kubernetesDeployer.setAppReplicas({
      app,
      replicas: 0,
      status: 'stopped',
    });

    return {
      app_id: updated.app_id,
      status: updated.status,
    };
  }

  async startApp({ owner, appId }: AppIdInput): Promise<StatusResponse> {
    const app = await this.kubernetesDeployer.findAppById(appId);

    if (!app || app.owner !== owner) {
      throw new ApiError(404, 'not_found', 'App not found');
    }

    const updated = await this.kubernetesDeployer.setAppReplicas({
      app,
      replicas: 1,
      status: 'deploying',
    });

    return {
      app_id: updated.app_id,
      status: updated.status,
    };
  }

  async deleteApp({ owner, appId }: AppIdInput): Promise<StatusResponse> {
    const app = await this.kubernetesDeployer.findAppById(appId);

    if (!app || app.owner !== owner) {
      throw new ApiError(404, 'not_found', 'App not found');
    }

    await this.kubernetesDeployer.deleteAppResources(app);
    await this.postgresProvisioner.dropSchema(app.app_id);

    return {
      app_id: app.app_id,
      status: 'deleting',
    };
  }

  async getLogs({ owner, appId, cursor, limit }: GetLogsInput): Promise<LogsPage> {
    const app = await this.kubernetesDeployer.findAppById(appId);

    if (!app || app.owner !== owner) {
      throw new ApiError(404, 'not_found', 'App not found');
    }

    return this.kubernetesDeployer.readLogs(app, cursor, limit);
  }
}

export function createAppService(): AppService {
  return new AppService({
    kubernetesDeployer: new KubernetesDeployer(),
    postgresProvisioner: new PostgresProvisioner(),
  });
}
