import React from 'react';
import { View, StyleSheet } from 'react-native';
import { Button, useTheme } from 'react-native-paper';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import Toast from 'react-native-toast-message';

interface BiometricAuthProps {
  onSuccess: () => void;
  style?: any;
}

const BiometricAuth: React.FC<BiometricAuthProps> = ({ onSuccess, style }) => {
  const theme = useTheme();

  const handleBiometricAuth = async () => {
    try {
      // In a real implementation, this would use react-native-biometrics
      Toast.show({
        type: 'info',
        text1: 'Biometric Authentication',
        text2: 'Feature coming soon - requires device setup',
      });
    } catch (error) {
      Toast.show({
        type: 'error',
        text1: 'Authentication Failed',
        text2: 'Could not authenticate with biometrics',
      });
    }
  };

  return (
    <View style={[styles.container, style]}>
      <Button
        mode="outlined"
        onPress={handleBiometricAuth}
        icon={() => (
          <Icon name="fingerprint" size={20} color={theme.colors.primary} />
        )}
        style={styles.button}
      >
        Use Biometrics
      </Button>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
  },
  button: {
    minWidth: 200,
  },
});

export default BiometricAuth;