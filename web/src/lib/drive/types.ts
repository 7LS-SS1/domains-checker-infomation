import { z } from "zod";

/** Mirrors internal/drive/service.go's Connection struct. Never carries a token. */
export const driveConnectionSchema = z.object({
  id: z.string(),
  user_id: z.string(),
  google_email: z.string().optional(),
  scopes: z.array(z.string()),
  status: z.string(),
  token_expires_at: z.string().nullish(),
  connected_at: z.string(),
  updated_at: z.string(),
});

export const driveAuthorizationSchema = z.object({
  authorization_url: z.string(),
  expires_at: z.string(),
});

const driveFileSchema = z.object({
  id: z.string(),
  name: z.string(),
  mime_type: z.string(),
  modified_time: z.string().optional(),
  web_view_link: z.string().optional(),
});

export const driveFilePageSchema = z.object({
  items: z.array(driveFileSchema),
  next_page_token: z.string().optional(),
});

export type DriveConnection = z.infer<typeof driveConnectionSchema>;
export type DriveFile = z.infer<typeof driveFileSchema>;
