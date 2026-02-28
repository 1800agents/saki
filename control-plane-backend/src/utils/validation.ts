import { z } from 'zod';

const APP_NAME_REGEX = /^(?=.{1,63}$)[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/;
const COMMIT_REGEX = /^[a-f0-9]{7,40}$/i;

export const preparePushSchema = z.object({
  name: z
    .string()
    .regex(
      APP_NAME_REGEX,
      'name must be DNS-safe (lowercase letters, digits, dash), max 63 chars, and alphanumeric at ends'
    ),
  git_commit: z
    .string()
    .regex(COMMIT_REGEX, 'git_commit must be a 7-40 character hexadecimal hash'),
});

export const upsertAppSchema = z.object({
  name: z
    .string()
    .regex(
      APP_NAME_REGEX,
      'name must be DNS-safe (lowercase letters, digits, dash), max 63 chars, and alphanumeric at ends'
    ),
  description: z.string().max(300, 'description must be <= 300 characters'),
  image: z.string().min(1, 'image is required'),
});

export class ValidationError extends Error {
  validationDetails: Record<string, unknown>;

  constructor(message: string, validationDetails: Record<string, unknown>) {
    super(message);
    this.validationDetails = validationDetails;
  }
}

export function parseOrThrow<T>(schema: z.ZodSchema<T>, payload: unknown): T {
  const parsed = schema.safeParse(payload);
  if (parsed.success) {
    return parsed.data;
  }

  const details = parsed.error.flatten() as unknown as Record<string, unknown>;
  const message = parsed.error.issues[0]?.message || 'Validation failed';
  throw new ValidationError(message, details);
}
