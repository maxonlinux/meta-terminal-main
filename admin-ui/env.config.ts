"use server";

import { cleanEnv, str, url } from "envalid";

export const getEnv = async () =>
  cleanEnv(process.env, {
    CORE_URL: url({ example: "http://localhost:3333/proxy/main" }),

    NODE_ENV: str({
      choices: ["development", "test", "production", "staging"],
      default: "development",
    }),
  });
