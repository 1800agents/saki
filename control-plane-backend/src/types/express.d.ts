export {};

declare global {
  namespace Express {
    interface Request {
      requestId: string;
      auth: {
        token: string;
        owner: string;
        isAdmin: boolean;
      };
    }
  }
}
