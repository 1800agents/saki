"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import styles from "./page.module.css";

type AppStatus = "pending" | "deploying" | "healthy" | "failed" | "stopped" | "deleting";

interface AppSummary {
  app_id: string;
  name: string;
  status: AppStatus;
  url: string;
}

interface AppDetail {
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

interface LogEntry {
  timestamp: string;
  stream: "stdout" | "stderr";
  message: string;
}

interface LogsPage {
  data: LogEntry[];
  next_cursor: string | null;
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
  status: AppStatus;
}

interface StatusResponse {
  app_id: string;
  status: AppStatus;
}

const DEFAULT_API_BASE = process.env.NEXT_PUBLIC_CONTROL_PLANE_API_BASE ?? "http://localhost:8080";

function normalizeApiBase(value: string): string {
  return value.trim().replace(/\/+$/, "");
}

function getApiErrorMessage(payload: unknown): string | null {
  if (!payload || typeof payload !== "object" || !("error" in payload)) {
    return null;
  }

  const errorValue = (payload as { error?: unknown }).error;
  if (!errorValue || typeof errorValue !== "object" || !("message" in errorValue)) {
    return null;
  }

  const message = (errorValue as { message?: unknown }).message;
  return typeof message === "string" ? message : null;
}

async function requestJson<T>(args: {
  apiBase: string;
  token: string;
  path: string;
  method?: "GET" | "POST" | "DELETE";
  params?: Record<string, string | undefined>;
  body?: unknown;
}): Promise<T> {
  const cleanBase = normalizeApiBase(args.apiBase);
  if (!cleanBase) {
    throw new Error("API base URL is required.");
  }

  if (!args.token.trim()) {
    throw new Error("Session token is required.");
  }

  const url = new URL(`${cleanBase}${args.path}`);
  url.searchParams.set("token", args.token.trim());

  if (args.params) {
    Object.entries(args.params).forEach(([key, value]) => {
      if (typeof value === "string" && value.length > 0) {
        url.searchParams.set(key, value);
      }
    });
  }

  const init: RequestInit = {
    method: args.method ?? "GET",
    headers: {},
  };

  if (args.body !== undefined) {
    (init.headers as Record<string, string>)["content-type"] = "application/json";
    init.body = JSON.stringify(args.body);
  }

  const response = await fetch(url.toString(), init);
  const text = await response.text();

  let parsed: unknown = null;
  if (text) {
    try {
      parsed = JSON.parse(text) as unknown;
    } catch {
      parsed = text;
    }
  }

  if (!response.ok) {
    const apiMessage = getApiErrorMessage(parsed);
    throw new Error(apiMessage ?? `Request failed with status ${response.status}`);
  }

  return parsed as T;
}

function formatTimestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString();
}

export default function Home() {
  const [apiBase, setApiBase] = useState<string>(DEFAULT_API_BASE);
  const [token, setToken] = useState<string>("");

  const [includeAll, setIncludeAll] = useState<boolean>(false);
  const [apps, setApps] = useState<AppSummary[]>([]);
  const [selectedAppId, setSelectedAppId] = useState<string | null>(null);
  const [selectedApp, setSelectedApp] = useState<AppDetail | null>(null);

  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);

  const [name, setName] = useState<string>("");
  const [description, setDescription] = useState<string>("");
  const [gitCommit, setGitCommit] = useState<string>("");
  const [image, setImage] = useState<string>("");
  const [prepareInfo, setPrepareInfo] = useState<PreparePushResponse | null>(null);

  const [isLoadingApps, setIsLoadingApps] = useState<boolean>(false);
  const [isLoadingApp, setIsLoadingApp] = useState<boolean>(false);
  const [isLoadingLogs, setIsLoadingLogs] = useState<boolean>(false);
  const [isPreparing, setIsPreparing] = useState<boolean>(false);
  const [isDeploying, setIsDeploying] = useState<boolean>(false);
  const [appActionId, setAppActionId] = useState<string | null>(null);

  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [noticeMessage, setNoticeMessage] = useState<string | null>(null);

  const hasSession = useMemo(
    () => normalizeApiBase(apiBase).length > 0 && token.trim().length > 0,
    [apiBase, token],
  );

  useEffect(() => {
    if (typeof window !== "undefined") {
      const queryToken = new URLSearchParams(window.location.search).get("token");
      const storedApiBase = window.localStorage.getItem("saki.apiBase");
      const storedToken = window.localStorage.getItem("saki.sessionToken");

      if (storedApiBase) {
        setApiBase(storedApiBase);
      }

      if (queryToken) {
        setToken(queryToken);
      } else if (storedToken) {
        setToken(storedToken);
      }
      return;
    }
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    window.localStorage.setItem("saki.apiBase", apiBase);
  }, [apiBase]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    if (token.trim()) {
      window.localStorage.setItem("saki.sessionToken", token.trim());
    } else {
      window.localStorage.removeItem("saki.sessionToken");
    }
  }, [token]);

  const loadApps = useCallback(async () => {
    if (!hasSession) {
      setApps([]);
      setSelectedAppId(null);
      setSelectedApp(null);
      setLogs([]);
      setNextCursor(null);
      return;
    }

    setIsLoadingApps(true);
    setErrorMessage(null);

    try {
      const response = await requestJson<{ data: AppSummary[] }>({
        apiBase,
        token,
        path: "/apps",
        params: includeAll ? { all: "true" } : undefined,
      });

      setApps(response.data);

      if (!response.data.length) {
        setSelectedAppId(null);
        setSelectedApp(null);
        setLogs([]);
        setNextCursor(null);
        return;
      }

      setSelectedAppId((current) => {
        if (current && response.data.some((app) => app.app_id === current)) {
          return current;
        }

        return response.data[0].app_id;
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to load apps.";
      setErrorMessage(message);
    } finally {
      setIsLoadingApps(false);
    }
  }, [apiBase, hasSession, includeAll, token]);

  const loadAppDetail = useCallback(
    async (appId: string) => {
      setIsLoadingApp(true);

      try {
        const response = await requestJson<AppDetail>({
          apiBase,
          token,
          path: `/apps/${encodeURIComponent(appId)}`,
        });

        setSelectedApp(response);
      } catch (error) {
        const message = error instanceof Error ? error.message : "Failed to load app detail.";
        setErrorMessage(message);
      } finally {
        setIsLoadingApp(false);
      }
    },
    [apiBase, token],
  );

  const loadAppLogs = useCallback(
    async (appId: string, cursor?: string, append = false) => {
      setIsLoadingLogs(true);

      try {
        const response = await requestJson<LogsPage>({
          apiBase,
          token,
          path: `/apps/${encodeURIComponent(appId)}/logs`,
          params: {
            limit: "200",
            cursor,
          },
        });

        setLogs((current) => (append ? current.concat(response.data) : response.data));
        setNextCursor(response.next_cursor);
      } catch (error) {
        const message = error instanceof Error ? error.message : "Failed to load logs.";
        setErrorMessage(message);
      } finally {
        setIsLoadingLogs(false);
      }
    },
    [apiBase, token],
  );

  useEffect(() => {
    if (!hasSession) {
      return;
    }

    void loadApps();
  }, [hasSession, loadApps]);

  useEffect(() => {
    if (!selectedAppId || !hasSession) {
      return;
    }

    void loadAppDetail(selectedAppId);
    void loadAppLogs(selectedAppId);
  }, [hasSession, loadAppDetail, loadAppLogs, selectedAppId]);

  const handlePrepare = async (event: FormEvent) => {
    event.preventDefault();

    if (!hasSession) {
      setErrorMessage("Set API base URL and token before preparing an image.");
      return;
    }

    setIsPreparing(true);
    setErrorMessage(null);
    setNoticeMessage(null);

    try {
      const response = await requestJson<PreparePushResponse>({
        apiBase,
        token,
        path: "/apps/prepare",
        method: "POST",
        body: {
          name,
          git_commit: gitCommit,
        },
      });

      setPrepareInfo(response);
      setImage(`${response.repository}:${response.required_tag}`);
      setNoticeMessage("Image destination prepared. Push your image and deploy.");
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to prepare image destination.";
      setErrorMessage(message);
    } finally {
      setIsPreparing(false);
    }
  };

  const handleDeploy = async (event: FormEvent) => {
    event.preventDefault();

    if (!hasSession) {
      setErrorMessage("Set API base URL and token before deploying.");
      return;
    }

    setIsDeploying(true);
    setErrorMessage(null);
    setNoticeMessage(null);

    try {
      const response = await requestJson<UpsertAppResponse>({
        apiBase,
        token,
        path: "/apps",
        method: "POST",
        body: {
          name,
          description,
          image,
        },
      });

      setNoticeMessage(`Deploy queued for ${name}.`);
      setSelectedAppId(response.app_id);
      await loadApps();
      await loadAppDetail(response.app_id);
      await loadAppLogs(response.app_id);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to deploy app.";
      setErrorMessage(message);
    } finally {
      setIsDeploying(false);
    }
  };

  const runAction = async (appId: string, action: "start" | "stop" | "delete") => {
    if (!hasSession) {
      return;
    }

    if (action === "delete") {
      const confirmed = window.confirm("Delete this app? This also drops its schema.");
      if (!confirmed) {
        return;
      }
    }

    setAppActionId(appId);
    setErrorMessage(null);
    setNoticeMessage(null);

    try {
      const path =
        action === "delete"
          ? `/apps/${encodeURIComponent(appId)}`
          : `/apps/${encodeURIComponent(appId)}/${action}`;

      await requestJson<StatusResponse>({
        apiBase,
        token,
        path,
        method: action === "delete" ? "DELETE" : "POST",
      });

      setNoticeMessage(`Action ${action} sent for app ${appId}.`);

      if (action === "delete") {
        setSelectedAppId((current) => (current === appId ? null : current));
      }

      await loadApps();
      if (selectedAppId && action !== "delete") {
        await loadAppDetail(selectedAppId);
        await loadAppLogs(selectedAppId);
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : `Failed to ${action} app.`;
      setErrorMessage(message);
    } finally {
      setAppActionId(null);
    }
  };

  const selectedSummary = useMemo(
    () => apps.find((app) => app.app_id === selectedAppId) ?? null,
    [apps, selectedAppId],
  );

  return (
    <main className={styles.page}>
      <div className={styles.hero}>
        <h1>Saki Control Plane</h1>
        <p>Deploy, observe, and operate apps against the live backend contract.</p>
      </div>

      <section className={styles.panel}>
        <h2>Session</h2>
        <div className={styles.fieldGrid}>
          <label className={styles.field}>
            <span>API base URL</span>
            <input
              value={apiBase}
              onChange={(event) => setApiBase(event.target.value)}
              placeholder="http://localhost:8080"
            />
          </label>

          <label className={styles.field}>
            <span>Session token (UUID)</span>
            <input
              value={token}
              onChange={(event) => setToken(event.target.value)}
              placeholder="11111111-1111-4111-8111-111111111111"
            />
          </label>
        </div>

        <div className={styles.inlineActions}>
          <button
            type="button"
            className={styles.actionButton}
            onClick={() => void loadApps()}
            disabled={!hasSession || isLoadingApps}
          >
            {isLoadingApps ? "Refreshing..." : "Refresh apps"}
          </button>

          <label className={styles.checkboxRow}>
            <input
              type="checkbox"
              checked={includeAll}
              onChange={(event) => setIncludeAll(event.target.checked)}
            />
            <span>Include all apps (admin tokens only)</span>
          </label>
        </div>
      </section>

      {errorMessage ? <div className={`${styles.notice} ${styles.error}`}>{errorMessage}</div> : null}
      {noticeMessage ? <div className={`${styles.notice} ${styles.ok}`}>{noticeMessage}</div> : null}

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>Deploy</h2>

          <form className={styles.form} onSubmit={handlePrepare}>
            <label className={styles.field}>
              <span>Name</span>
              <input
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="my-app"
                required
              />
            </label>

            <label className={styles.field}>
              <span>Description</span>
              <textarea
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                maxLength={300}
                placeholder="Internal app for demos"
                required
              />
            </label>

            <label className={styles.field}>
              <span>Git commit (for prepare)</span>
              <input
                value={gitCommit}
                onChange={(event) => setGitCommit(event.target.value)}
                placeholder="b7c1a2f5d8e9c0a1b2c3d4e5f6a7b8c9d0e1f2a3"
                required
              />
            </label>

            <button type="submit" className={styles.actionButton} disabled={!hasSession || isPreparing}>
              {isPreparing ? "Preparing..." : "Prepare image destination"}
            </button>
          </form>

          {prepareInfo ? (
            <div className={styles.prepareCard}>
              <p>
                <strong>Repository:</strong> {prepareInfo.repository}
              </p>
              <p>
                <strong>Required tag:</strong> {prepareInfo.required_tag}
              </p>
              <p>
                <strong>Push token expires:</strong> {formatTimestamp(prepareInfo.expires_at)}
              </p>
            </div>
          ) : null}

          <form className={styles.form} onSubmit={handleDeploy}>
            <label className={styles.field}>
              <span>Image</span>
              <input
                value={image}
                onChange={(event) => setImage(event.target.value)}
                placeholder="registry.internal/<owner>/my-app:b7c1a2f"
                required
              />
            </label>

            <button type="submit" className={styles.actionButton} disabled={!hasSession || isDeploying}>
              {isDeploying ? "Deploying..." : "Deploy / Redeploy"}
            </button>
          </form>
        </section>

        <section className={styles.panel}>
          <h2>Apps</h2>
          <div className={styles.appList}>
            {apps.length ? null : <p className={styles.empty}>No apps yet.</p>}
            {apps.map((app) => (
              <button
                key={app.app_id}
                type="button"
                onClick={() => setSelectedAppId(app.app_id)}
                className={`${styles.appRow} ${app.app_id === selectedAppId ? styles.appRowActive : ""}`}
              >
                <span className={styles.appName}>{app.name}</span>
                <span className={`${styles.status} ${styles[`status_${app.status}`]}`}>{app.status}</span>
                <span className={styles.url}>{app.url}</span>
              </button>
            ))}
          </div>
        </section>
      </div>

      <section className={styles.panel}>
        <div className={styles.detailHeader}>
          <h2>Selected app</h2>
          {selectedSummary ? (
            <div className={styles.inlineActions}>
              <button
                type="button"
                className={styles.actionButton}
                onClick={() => void runAction(selectedSummary.app_id, "start")}
                disabled={appActionId === selectedSummary.app_id}
              >
                Start
              </button>
              <button
                type="button"
                className={styles.actionButton}
                onClick={() => void runAction(selectedSummary.app_id, "stop")}
                disabled={appActionId === selectedSummary.app_id}
              >
                Stop
              </button>
              <button
                type="button"
                className={`${styles.actionButton} ${styles.danger}`}
                onClick={() => void runAction(selectedSummary.app_id, "delete")}
                disabled={appActionId === selectedSummary.app_id}
              >
                Delete
              </button>
            </div>
          ) : null}
        </div>

        {!selectedSummary ? <p className={styles.empty}>Select an app to inspect status and logs.</p> : null}

        {selectedApp ? (
          <div className={styles.metaGrid}>
            <p>
              <strong>ID:</strong> {selectedApp.app_id}
            </p>
            <p>
              <strong>Deployment:</strong> {selectedApp.deployment_id}
            </p>
            <p>
              <strong>Status:</strong>{" "}
              <span className={`${styles.status} ${styles[`status_${selectedApp.status}`]}`}>{selectedApp.status}</span>
            </p>
            <p>
              <strong>URL:</strong> {selectedApp.url}
            </p>
            <p>
              <strong>TTL expiry:</strong> {formatTimestamp(selectedApp.ttl_expiry)}
            </p>
            <p>
              <strong>Updated:</strong> {formatTimestamp(selectedApp.updated_at)}
            </p>
            <p className={styles.metaWide}>
              <strong>Image:</strong> {selectedApp.image}
            </p>
            <p className={styles.metaWide}>
              <strong>Description:</strong> {selectedApp.description}
            </p>
          </div>
        ) : isLoadingApp ? (
          <p className={styles.empty}>Loading app details...</p>
        ) : null}

        <div className={styles.logsHeader}>
          <h3>Logs</h3>
          <div className={styles.inlineActions}>
            <button
              type="button"
              className={styles.actionButton}
              onClick={() => (selectedAppId ? void loadAppLogs(selectedAppId) : undefined)}
              disabled={!selectedAppId || isLoadingLogs}
            >
              {isLoadingLogs ? "Loading..." : "Refresh logs"}
            </button>
            <button
              type="button"
              className={styles.actionButton}
              onClick={() =>
                selectedAppId && nextCursor ? void loadAppLogs(selectedAppId, nextCursor, true) : undefined
              }
              disabled={!selectedAppId || !nextCursor || isLoadingLogs}
            >
              Load more
            </button>
          </div>
        </div>

        <div className={styles.logBox}>
          {logs.length ? null : <p className={styles.empty}>No log lines returned.</p>}
          {logs.map((line, index) => (
            <div key={`${line.timestamp}-${line.message}-${index}`} className={styles.logLine}>
              <span className={styles.logTime}>{formatTimestamp(line.timestamp)}</span>
              <span className={`${styles.logStream} ${line.stream === "stderr" ? styles.stderr : styles.stdout}`}>
                {line.stream}
              </span>
              <span>{line.message}</span>
            </div>
          ))}
        </div>
      </section>
    </main>
  );
}
