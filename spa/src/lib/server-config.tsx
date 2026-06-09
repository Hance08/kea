import { useQuery } from '@tanstack/react-query';
import { type ReactNode, createContext, useContext } from 'react';
import { getConfig } from './api';
import type { ServerConfig } from './types';

const ServerConfigContext = createContext<ServerConfig | null>(null);

interface ServerConfigProviderProps {
  fallback: ReactNode;
  children: (cfg: ServerConfig) => ReactNode;
}

export function ServerConfigProvider({ fallback, children }: ServerConfigProviderProps) {
  const query = useQuery({
    queryKey: ['server-config'],
    queryFn: getConfig,
    staleTime: Number.POSITIVE_INFINITY,
  });

  if (query.isPending || query.isError) {
    return <>{fallback}</>;
  }

  return (
    <ServerConfigContext.Provider value={query.data}>
      {children(query.data)}
    </ServerConfigContext.Provider>
  );
}

export function useServerConfig(): ServerConfig {
  const cfg = useContext(ServerConfigContext);
  if (!cfg) {
    throw new Error('useServerConfig must be used inside ServerConfigProvider');
  }
  return cfg;
}
