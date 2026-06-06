/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_DEFAULT_CURRENCY?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
