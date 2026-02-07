import React from 'react';
import { View, StyleSheet } from 'react-native';
import { ActivityIndicator, Text, useTheme } from 'react-native-paper';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

const LoadingScreen: React.FC = () => {
  const theme = useTheme();

  return (
    <View style={[styles.container, { backgroundColor: theme.colors.background }]}>
      <Icon 
        name="shield-key" 
        size={64} 
        color={theme.colors.primary} 
        style={styles.icon}
      />
      <Text variant="headlineSmall" style={styles.title}>
        Coldforge Vault
      </Text>
      <ActivityIndicator 
        size="large" 
        color={theme.colors.primary} 
        style={styles.loader}
      />
      <Text variant="bodyMedium" style={styles.subtitle}>
        Securing your data...
      </Text>
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    paddingHorizontal: 24,
  },
  icon: {
    marginBottom: 16,
  },
  title: {
    fontWeight: '600',
    marginBottom: 32,
  },
  loader: {
    marginBottom: 16,
  },
  subtitle: {
    color: '#64748b',
  },
});

export default LoadingScreen;