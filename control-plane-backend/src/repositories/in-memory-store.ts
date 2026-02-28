import { randomUUID } from 'node:crypto';
import type { AppRecord, AppStatus, LogEntry } from '../types/app';

interface UpsertInput {
  owner: string;
  name: string;
  description: string;
  image: string;
  url: string;
  ttlExpiry: string;
}

export class InMemoryStore {
  private appsById = new Map<string, AppRecord>();
  private indexByOwnerAndName = new Map<string, string>();
  private logsByAppId = new Map<string, LogEntry[]>();

  private static ownerKey(owner: string, name: string): string {
    return `${owner}:${name}`;
  }

  upsert({ owner, name, description, image, url, ttlExpiry }: UpsertInput): AppRecord {
    const key = InMemoryStore.ownerKey(owner, name);
    const existingAppId = this.indexByOwnerAndName.get(key);
    const now = new Date().toISOString();

    if (existingAppId) {
      const current = this.appsById.get(existingAppId);
      if (!current) {
        throw new Error('Store index is inconsistent');
      }

      const next: AppRecord = {
        ...current,
        description,
        image,
        url,
        status: 'deploying',
        updated_at: now,
      };

      this.appsById.set(existingAppId, next);
      this.addLog(existingAppId, 'stdout', `Redeploy requested with image ${image}`);
      return next;
    }

    const app_id = `app_${randomUUID().replace(/-/g, '').slice(0, 12)}`;
    const deployment_id = `dep_${randomUUID().replace(/-/g, '').slice(0, 12)}`;

    const app: AppRecord = {
      app_id,
      deployment_id,
      owner,
      name,
      description,
      image,
      url,
      status: 'deploying',
      created_at: now,
      updated_at: now,
      ttl_expiry: ttlExpiry,
    };

    this.appsById.set(app_id, app);
    this.indexByOwnerAndName.set(key, app_id);
    this.logsByAppId.set(app_id, []);
    this.addLog(app_id, 'stdout', `App created with image ${image}`);

    return app;
  }

  markStatus(appId: string, status: AppStatus): AppRecord | null {
    const current = this.appsById.get(appId);
    if (!current) {
      return null;
    }

    const next: AppRecord = {
      ...current,
      status,
      updated_at: new Date().toISOString(),
    };

    this.appsById.set(appId, next);
    this.addLog(appId, 'stdout', `Status changed to ${status}`);
    return next;
  }

  getById(appId: string): AppRecord | null {
    return this.appsById.get(appId) || null;
  }

  listByOwner(owner: string): AppRecord[] {
    return [...this.appsById.values()].filter((app) => app.owner === owner);
  }

  listAll(): AppRecord[] {
    return [...this.appsById.values()];
  }

  delete(appId: string): AppRecord | null {
    const current = this.appsById.get(appId);
    if (!current) {
      return null;
    }

    this.appsById.delete(appId);
    this.indexByOwnerAndName.delete(InMemoryStore.ownerKey(current.owner, current.name));
    this.addLog(appId, 'stdout', 'Delete requested');

    return current;
  }

  addLog(appId: string, stream: 'stdout' | 'stderr', message: string): void {
    const logs = this.logsByAppId.get(appId) || [];
    logs.push({
      timestamp: new Date().toISOString(),
      stream,
      message,
    });
    this.logsByAppId.set(appId, logs);
  }

  getLogs(appId: string): LogEntry[] {
    return this.logsByAppId.get(appId) || [];
  }
}
