import { z } from "zod";

export const authUserSchema = z.object({
  id: z.string(),
  email: z.string(),
  displayName: z.string(),
  locale: z.string(),
  roles: z.array(z.string()),
});

export type AuthUser = z.infer<typeof authUserSchema>;

export const loginDataSchema = z.object({
  user: authUserSchema,
  expiresAt: z.string(),
});

export const logoutDataSchema = z.object({
  loggedOut: z.boolean(),
});
