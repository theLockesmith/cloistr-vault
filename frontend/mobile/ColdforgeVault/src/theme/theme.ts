import { MD3LightTheme, MD3DarkTheme } from 'react-native-paper';

const lightTheme = {
  ...MD3LightTheme,
  colors: {
    ...MD3LightTheme.colors,
    primary: '#2563eb',
    primaryContainer: '#dbeafe',
    secondary: '#64748b',
    secondaryContainer: '#f1f5f9',
    background: '#ffffff',
    surface: '#f8fafc',
    surfaceVariant: '#f1f5f9',
    error: '#dc2626',
    onError: '#ffffff',
  },
};

const darkTheme = {
  ...MD3DarkTheme,
  colors: {
    ...MD3DarkTheme.colors,
    primary: '#3b82f6',
    primaryContainer: '#1e40af',
    secondary: '#94a3b8',
    secondaryContainer: '#334155',
    background: '#0f172a',
    surface: '#1e293b',
    surfaceVariant: '#334155',
    error: '#ef4444',
    onError: '#ffffff',
  },
};

export const theme = lightTheme; // You can implement theme switching
export { lightTheme, darkTheme };