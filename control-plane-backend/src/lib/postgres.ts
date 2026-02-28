import { Pool } from 'pg';
import { config } from '../config/env';
import type { AppRecord } from '../types/app';

export class PostgresProvisioner {
  private readonly pool: Pool | null;

  constructor() {
    this.pool = config.postgresUrl
      ? new Pool({
          connectionString: config.postgresUrl,
        })
      : null;
  }

  private static schemaName(app: AppRecord): string {
    return `app_${app.app_id}`;
  }

  async ensureSchema(app: AppRecord): Promise<void> {
    if (!this.pool) {
      return;
    }

    const schema = PostgresProvisioner.schemaName(app);
    await this.pool.query(`CREATE SCHEMA IF NOT EXISTS "${schema}"`);
  }

  async dropSchema(app: AppRecord): Promise<void> {
    if (!this.pool) {
      return;
    }

    const schema = PostgresProvisioner.schemaName(app);
    await this.pool.query(`DROP SCHEMA IF EXISTS "${schema}" CASCADE`);
  }
}
