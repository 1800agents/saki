import type { ErrorRequestHandler, RequestHandler } from 'express';

export class ApiError extends Error {
  status: number;
  code: string;
  details: Record<string, unknown>;

  constructor(status: number, code: string, message: string, details: Record<string, unknown> = {}) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export const notFoundHandler: RequestHandler = (_req, _res, next) => {
  next(new ApiError(404, 'not_found', 'Route not found'));
};

export const apiErrorHandler: ErrorRequestHandler = (err, _req, res, _next) => {
  if (err instanceof ApiError) {
    return res.status(err.status).json({
      error: {
        code: err.code,
        message: err.message,
        details: err.details,
      },
    });
  }

  console.error(err);

  return res.status(500).json({
    error: {
      code: 'internal_error',
      message: 'Internal server error',
      details: {},
    },
  });
};
