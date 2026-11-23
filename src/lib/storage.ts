import { readFile } from "fs/promises";
import { getSignedUrl } from "@aws-sdk/s3-request-presigner";
import { S3Client, GetObjectCommand, PutObjectCommand, ListObjectsV2Command, DeleteObjectsCommand } from "@aws-sdk/client-s3";

export const storageEnabled = !!(process.env.S3_BUCKET && process.env.S3_ACCESS_KEY_ID && process.env.S3_SECRET_ACCESS_KEY);

let client: S3Client | undefined;
const s3 = () => (client ??= new S3Client({ region: process.env.S3_REGION ?? "auto", endpoint: process.env.S3_ENDPOINT, forcePathStyle: !!process.env.S3_FORCE_PATH_STYLE, credentials: { accessKeyId: process.env.S3_ACCESS_KEY_ID!, secretAccessKey: process.env.S3_SECRET_ACCESS_KEY! } }));

export const privateMedia = storageEnabled && process.env.S3_PRIVATE !== "0";

export const signedVideoUrl = (key: string, seconds = 3600) =>
  getSignedUrl(s3(), new GetObjectCommand({ Bucket: process.env.S3_BUCKET, Key: key, ResponseContentDisposition: 'attachment; filename="adreel-video.mp4"' }), { expiresIn: seconds });

export async function uploadVideo(path: string, key: string) {
  await s3().send(new PutObjectCommand({ Bucket: process.env.S3_BUCKET, Key: key, Body: await readFile(path), ContentType: "video/mp4", CacheControl: "public, max-age=31536000, immutable" }));
  return privateMedia ? key : `${process.env.S3_PUBLIC_URL!.replace(/\/$/, "")}/${key}`;
}

export async function deletePrefix(prefix: string) {
  if (!storageEnabled) return;
  let token: string | undefined;
  do {
    const r = await s3().send(new ListObjectsV2Command({ Bucket: process.env.S3_BUCKET, Prefix: prefix, ContinuationToken: token }));
    const keys = (r.Contents ?? []).map((o) => ({ Key: o.Key! }));
    if (keys.length) await s3().send(new DeleteObjectsCommand({ Bucket: process.env.S3_BUCKET, Delete: { Objects: keys } }));
    token = r.NextContinuationToken;
  } while (token);
}
