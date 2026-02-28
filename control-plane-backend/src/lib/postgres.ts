import { Pool } from 'pg';
import { config } from '../config/env';

export class PostgresProvisioner {
  private readonly pool: Pool | null;

  constructor() {
    this.pool = config.postgresPassword
      ? new Pool({
          host: config.postgresHost,
          port: config.postgresPort,
          user: config.postgresUser,
          password: config.postgresPassword,
          database: config.postgresDatabase,
        })
      : null;
  }

  private static schemaName(appId: string): string {
    return `app_${appId.replace(/[^a-zA-Z0-9_]/g, '_')}`;
  }

  databaseUrlForApp(appId: string): string {
    const schema = PostgresProvisioner.schemaName(appId);
    const url = new URL('postgresql://localhost');

    url.username = config.postgresUser;
    if (config.postgresPassword) {
      url.password = config.postgresPassword;
    }
    url.hostname = config.postgresHost;
    url.port = String(config.postgresPort);
    url.pathname = `/${config.postgresDatabase}`;
    url.searchParams.set('options', `-csearch_path=${schema}`);

    return url.toString();
  }

  async ensureSchema(appId: string): Promise<void> {
    if (!this.pool) {
      return;
    }

    const schema = PostgresProvisioner.schemaName(appId);
    await this.pool.query(`CREATE SCHEMA IF NOT EXISTS "${schema}"`);
  }

  async dropSchema(appId: string): Promise<void> {
    if (!this.pool) {
      return;
    }

    const schema = PostgresProvisioner.schemaName(appId);
    await this.pool.query(`DROP SCHEMA IF EXISTS "${schema}" CASCADE`);
  }
}
