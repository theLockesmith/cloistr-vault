import React from 'react';
import { NavigationContainer } from '@react-navigation/native';
import { PaperProvider } from 'react-native-paper';
import Toast from 'react-native-toast-message';
import { AuthProvider } from './src/contexts/AuthContext';
import { CryptoProvider } from './src/contexts/CryptoContext';
import AppNavigator from './src/navigation/AppNavigator';
import { theme } from './src/theme/theme';

const App: React.FC = () => {
  return (
    <PaperProvider theme={theme}>
      <NavigationContainer>
        <AuthProvider>
          <CryptoProvider>
            <AppNavigator />
            <Toast />
          </CryptoProvider>
        </AuthProvider>
      </NavigationContainer>
    </PaperProvider>
  );
};

export default App;