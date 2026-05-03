import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';

export type ThemeMode = 'light' | 'dark';

const THEME_KEY = 'papertrader.theme';

const DEFAULT_THEME: ThemeMode = 'light';

interface ThemeContextValue {
  theme: ThemeMode;
  setTheme: (mode: ThemeMode) => void;
  toggleTheme: () => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

const readStored = <T extends string>(key: string, fallback: T, allowed: readonly T[]): T => {
  try {
    const raw = window.localStorage.getItem(key);
    if (raw && (allowed as readonly string[]).includes(raw)) {
      return raw as T;
    }
  } catch {
    // ignore (SSR / privacy mode)
  }
  return fallback;
};

const writeStored = (key: string, value: string) => {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // ignore
  }
};

export const ThemeProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [theme, setThemeState] = useState<ThemeMode>(() =>
    readStored<ThemeMode>(THEME_KEY, DEFAULT_THEME, ['light', 'dark'] as const)
  );

  useEffect(() => {
    const root = document.documentElement;
    root.dataset.theme = theme;
    document.body.dataset.theme = theme;
  }, [theme]);

  const setTheme = useCallback((mode: ThemeMode) => {
    setThemeState(mode);
    writeStored(THEME_KEY, mode);
  }, []);

  const toggleTheme = useCallback(() => {
    setThemeState((prev) => {
      const next: ThemeMode = prev === 'light' ? 'dark' : 'light';
      writeStored(THEME_KEY, next);
      return next;
    });
  }, []);

  const value = useMemo<ThemeContextValue>(
    () => ({ theme, setTheme, toggleTheme }),
    [theme, setTheme, toggleTheme]
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
};

export const useTheme = (): ThemeContextValue => {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error('useTheme must be used within a ThemeProvider');
  }
  return ctx;
};
