import express from 'express';
import { requireSessionToken } from '../middleware/auth';
import { ApiError } from '../middleware/error-handler';
import { parseOrThrow, preparePushSchema, upsertAppSchema, ValidationError } from '../utils/validation';
import type { AppService } from '../services/apps.service';

export function buildAppsRouter(appService: AppService) {
  const router = express.Router();

  router.use(requireSessionToken);

  router.post('/prepare', (req, res, next) => {
    try {
      const payload = parseOrThrow(preparePushSchema, req.body);

      const response = appService.preparePush({
        owner: req.auth.owner,
        name: payload.name,
        gitCommit: payload.git_commit,
      });

      res.json(response);
    } catch (err) {
      if (err instanceof ValidationError) {
        return next(new ApiError(400, 'validation_error', err.message, err.validationDetails));
      }

      return next(err);
    }
  });

  router.post('/', async (req, res, next) => {
    try {
      const payload = parseOrThrow(upsertAppSchema, req.body);

      const response = await appService.upsertApp({
        owner: req.auth.owner,
        name: payload.name,
        description: payload.description,
        image: payload.image,
      });

      res.json(response);
    } catch (err) {
      if (err instanceof ValidationError) {
        return next(new ApiError(400, 'validation_error', err.message, err.validationDetails));
      }

      return next(err);
    }
  });

  router.get('/', (req, res, next) => {
    try {
      const includeAll = req.query.all === 'true';
      const response = appService.listApps({
        owner: req.auth.owner,
        includeAll,
        isAdmin: req.auth.isAdmin,
      });
      res.json(response);
    } catch (err) {
      next(err);
    }
  });

  router.get('/:appId/logs', async (req, res, next) => {
    try {
      const requestedLimit = Number(req.query.limit);
      const limit = Number.isFinite(requestedLimit)
        ? Math.min(Math.max(Math.trunc(requestedLimit), 1), 1000)
        : 200;

      const response = await appService.getLogs({
        owner: req.auth.owner,
        appId: req.params.appId,
        cursor: typeof req.query.cursor === 'string' ? req.query.cursor : undefined,
        limit,
      });

      res.json(response);
    } catch (err) {
      next(err);
    }
  });

  router.get('/:appId', (req, res, next) => {
    try {
      const response = appService.getApp({
        owner: req.auth.owner,
        appId: req.params.appId,
      });

      res.json(response);
    } catch (err) {
      next(err);
    }
  });

  router.post('/:appId/stop', async (req, res, next) => {
    try {
      const response = await appService.stopApp({
        owner: req.auth.owner,
        appId: req.params.appId,
      });

      res.json(response);
    } catch (err) {
      next(err);
    }
  });

  router.post('/:appId/start', async (req, res, next) => {
    try {
      const response = await appService.startApp({
        owner: req.auth.owner,
        appId: req.params.appId,
      });

      res.json(response);
    } catch (err) {
      next(err);
    }
  });

  router.delete('/:appId', async (req, res, next) => {
    try {
      const response = await appService.deleteApp({
        owner: req.auth.owner,
        appId: req.params.appId,
      });

      res.json(response);
    } catch (err) {
      next(err);
    }
  });

  return router;
}
