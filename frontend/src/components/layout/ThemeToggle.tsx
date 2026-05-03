import React from 'react';
import { useTheme } from '../../hooks/useTheme';

const SunIcon: React.FC = () => (
  <svg className="ico" viewBox="0 0 16 16" aria-hidden="true">
    <circle cx="8" cy="8" r="3" />
    <path d="M8 1.5v1.5M8 13v1.5M1.5 8h1.5M13 8h1.5M3.5 3.5l1 1M11.5 11.5l1 1M3.5 12.5l1-1M11.5 4.5l1-1" />
  </svg>
);

const MoonIcon: React.FC = () => (
  <svg className="ico" viewBox="0 0 16 16" aria-hidden="true">
    <path d="M13 9.5A5.5 5.5 0 016.5 3a5.5 5.5 0 106.5 6.5z" />
  </svg>
);

const ThemeToggle: React.FC = () => {
  const { theme, toggleTheme } = useTheme();
  const next = theme === 'light' ? 'dark' : 'light';

  return (
    <button
      type="button"
      className="theme-toggle"
      onClick={toggleTheme}
      aria-label={`Switch to ${next} mode`}
      title={`Switch to ${next} mode`}
    >
      {theme === 'light' ? <MoonIcon /> : <SunIcon />}
    </button>
  );
};

export default ThemeToggle;
