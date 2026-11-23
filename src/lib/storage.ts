import { readFile } from "fs/promises";
import { S3Client, PutObjectCommand, ListObjectsV2Command, DeleteObjectsCommand } from "@aws-sdk/client-s3";

export const storageEnabled = !!(process.env.S3_BUCKET && process.env.S3_ACCESS_KEY_ID && process.env.S3_SECRET_ACCESS_KEY && process.env.S3_PUBLIC_URL);

let client: S3Client | undefined;
const s3 = () => (client ??= new S3Client({ region: process.env.S3_REGION ?? "auto", endpoint: process.env.S3_ENDPOINT, forcePathStyle: !!process.env.S3_FORCE_PATH_STYLE, credentials: { accessKeyId: process.env.S3_ACCESS_KEY_ID!, secretAccessKey: process.env.S3_SECRET_ACCESS_KEY! } }));

export async function uploadVideo(path: string, key: string) {
  await s3().send(new PutObjectCommand({ Bucket: process.env.S3_BUCKET, Key: key, Body: await readFile(path), ContentType: "video/mp4", CacheControl: "public, max-age=31536000, immutable" }));
  return `${process.env.S3_PUBLIC_URL!.replace(/\/$/, "")}/${key}`;
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
