import express from 'express';
import cors from 'cors';
import helmet from 'helmet';
import morgan from 'morgan';
import { randomUUID } from 'node:crypto';
import { buildAppsRouter } from './routes/apps.routes';
import { createAppService } from './services/apps.service';
import { apiErrorHandler, notFoundHandler } from './middleware/error-handler';

export function createApp() {
  const app = express();
  const appService = createAppService();

  app.use(helmet());
  app.use(cors());
  app.use(express.json({ limit: '1mb' }));

  app.use((req, res, next) => {
    req.requestId = randomUUID();
    res.setHeader('x-request-id', req.requestId);
    next();
  });

  app.use(morgan('combined'));

  app.get('/healthz', (_req, res) => {
    res.json({ status: 'ok' });
  });

  app.use('/apps', buildAppsRouter(appService));

  app.use(notFoundHandler);
  app.use(apiErrorHandler);

  return app;
}
