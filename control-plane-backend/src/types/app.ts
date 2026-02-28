export type AppStatus = 'pending' | 'deploying' | 'healthy' | 'failed' | 'stopped' | 'deleting';

export interface AppRecord {
  app_id: string;
  deployment_id: string;
  owner: string;
  name: string;
  description: string;
  image: string;
  url: string;
  status: AppStatus;
  created_at: string;
  updated_at: string;
  ttl_expiry: string;
}

export interface LogEntry {
  timestamp: string;
  stream: 'stdout' | 'stderr';
  message: string;
}

export interface LogsPage {
  data: LogEntry[];
  next_cursor: string | null;
}

export interface AppSummary {
  app_id: string;
  name: string;
  status: AppStatus;
  url: string;
}

export interface AppDetail {
  app_id: string;
  deployment_id: string;
  name: string;
  description: string;
  url: string;
  status: AppStatus;
  created_at: string;
  updated_at: string;
  image: string;
  ttl_expiry: string;
  owner: string;
}
