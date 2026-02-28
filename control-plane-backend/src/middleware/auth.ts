import type { RequestHandler } from 'express';
import { ApiError } from './error-handler';
import { config } from '../config/env';

const UUID_REGEX =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export const requireSessionToken: RequestHandler = (req, _res, next) => {
  const token = req.query.token;

  if (!token || typeof token !== 'string') {
    return next(
      new ApiError(
        401,
        'invalid_session',
        'Missing session token. Supply ?token=<session_uuid>.'
      )
    );
  }

  if (!UUID_REGEX.test(token)) {
    return next(new ApiError(401, 'invalid_session', 'Session token must be a valid UUID.'));
  }

  req.auth = {
    token,
    owner: token,
    isAdmin: config.adminTokens.has(token),
  };

  return next();
};
