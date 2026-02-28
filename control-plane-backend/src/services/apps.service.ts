import { randomUUID } from 'node:crypto';
import { ApiError } from '../middleware/error-handler';
import { InMemoryStore } from '../repositories/in-memory-store';
import { KubernetesDeployer } from '../lib/kubernetes';
import { PostgresProvisioner } from '../lib/postgres';
import { config } from '../config/env';
import type { AppDetail, AppRecord, AppSummary, LogsPage } from '../types/app';

interface AppServiceDeps {
  store: InMemoryStore;
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
  private readonly store: InMemoryStore;
  private readonly kubernetesDeployer: KubernetesDeployer;
  private readonly postgresProvisioner: PostgresProvisioner;

  constructor({ store, kubernetesDeployer, postgresProvisioner }: AppServiceDeps) {
    this.store = store;
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
    const app = this.store.upsert({
      owner,
      name,
      description,
      image,
      url,
      ttlExpiry: computeTtlExpiry(),
    });

    await this.postgresProvisioner.ensureSchema(app);
    await this.kubernetesDeployer.deployApp(app);

    return {
      app_id: app.app_id,
      deployment_id: app.deployment_id,
      url: app.url,
      status: app.status,
    };
  }

  getApp({ owner, appId }: AppIdInput): AppDetail {
    const app = this.store.getById(appId);

    if (!app || app.owner !== owner) {
      throw new ApiError(404, 'not_found', 'App not found');
    }

    return toAppDetail(app);
  }

  listApps({ owner, includeAll, isAdmin }: ListAppsInput): ListAppsResponse {
    if (includeAll && !isAdmin) {
      throw new ApiError(403, 'forbidden', 'Listing all apps requires admin access');
    }

    const apps = includeAll ? this.store.listAll() : this.store.listByOwner(owner);

    return {
      data: apps.map(toAppSummary),
    };
  }

  async stopApp({ owner, appId }: AppIdInput): Promise<StatusResponse> {
    const app = this.store.getById(appId);

    if (!app || app.owner !== owner) {
      throw new ApiError(404, 'not_found', 'App not found');
    }

    await this.kubernetesDeployer.stopApp(app);
    const updated = this.store.markStatus(appId, 'stopped');

    if (!updated) {
      throw new ApiError(500, 'internal_error', 'Unable to update app status');
    }

    return {
      app_id: updated.app_id,
      status: updated.status,
    };
  }

  async startApp({ owner, appId }: AppIdInput): Promise<StatusResponse> {
    const app = this.store.getById(appId);

    if (!app || app.owner !== owner) {
      throw new ApiError(404, 'not_found', 'App not found');
    }

    await this.kubernetesDeployer.startApp(app);
    const updated = this.store.markStatus(appId, 'deploying');

    if (!updated) {
      throw new ApiError(500, 'internal_error', 'Unable to update app status');
    }

    return {
      app_id: updated.app_id,
      status: updated.status,
    };
  }

  async deleteApp({ owner, appId }: AppIdInput): Promise<StatusResponse> {
    const app = this.store.getById(appId);

    if (!app || app.owner !== owner) {
      throw new ApiError(404, 'not_found', 'App not found');
    }

    this.store.markStatus(appId, 'deleting');
    await this.kubernetesDeployer.deleteApp(app);
    await this.postgresProvisioner.dropSchema(app);
    const deleted = this.store.delete(appId);

    if (!deleted) {
      throw new ApiError(500, 'internal_error', 'Unable to delete app');
    }

    return {
      app_id: deleted.app_id,
      status: 'deleting',
    };
  }

  async getLogs({ owner, appId, cursor, limit }: GetLogsInput): Promise<LogsPage> {
    const app = this.store.getById(appId);

    if (!app || app.owner !== owner) {
      throw new ApiError(404, 'not_found', 'App not found');
    }

    const localLogs = this.store.getLogs(appId);
    const cursorOffset = cursor ? Number(cursor) : 0;
    const safeOffset = Number.isFinite(cursorOffset) && cursorOffset > 0 ? cursorOffset : 0;

    const page = localLogs.slice(safeOffset, safeOffset + limit);
    const nextOffset = safeOffset + page.length;

    const clusterLogs = await this.kubernetesDeployer.readLogs(app, cursor, limit);

    return {
      data: page.concat(clusterLogs.data),
      next_cursor:
        nextOffset < localLogs.length ? String(nextOffset) : clusterLogs.next_cursor || null,
    };
  }
}

export function createAppService(): AppService {
  return new AppService({
    store: new InMemoryStore(),
    kubernetesDeployer: new KubernetesDeployer(),
    postgresProvisioner: new PostgresProvisioner(),
  });
}
