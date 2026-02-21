"use server";

import { cleanEnv, str, url } from "envalid";

export const getEnv = async () =>
  cleanEnv(process.env, {
    BASE_URL: url({ example: "http://localhost:3333" }),

    NODE_ENV: str({
      choices: ["development", "test", "production", "staging"],
      default: "development",
    }),
  });
