import React from 'react';
import { View, StyleSheet, ScrollView } from 'react-native';
import { Text, Card, FAB, useTheme } from 'react-native-paper';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import { useAuth } from '../../contexts/AuthContext';

const VaultScreen: React.FC = () => {
  const theme = useTheme();
  const { user } = useAuth();

  return (
    <View style={styles.container}>
      <ScrollView contentContainerStyle={styles.scrollContent}>
        <Card style={styles.welcomeCard}>
          <Card.Content style={styles.welcomeContent}>
            <Icon name="shield-check" size={48} color={theme.colors.primary} />
            <Text variant="headlineSmall" style={styles.welcomeTitle}>
              Welcome to Your Vault
            </Text>
            <Text variant="bodyMedium" style={styles.welcomeText}>
              Your passwords and secrets are securely encrypted.
            </Text>
          </Card.Content>
        </Card>

        <Card style={styles.emptyCard}>
          <Card.Content style={styles.emptyContent}>
            <Icon name="folder-key" size={64} color={theme.colors.outline} />
            <Text variant="titleMedium" style={styles.emptyTitle}>
              No items yet
            </Text>
            <Text variant="bodyMedium" style={styles.emptyText}>
              Tap the + button to add your first password, note, or secure item.
            </Text>
          </Card.Content>
        </Card>

        <Card style={styles.infoCard}>
          <Card.Content>
            <View style={styles.infoRow}>
              <Icon name="lock" size={20} color={theme.colors.primary} />
              <Text variant="bodySmall" style={styles.infoText}>
                End-to-end encrypted
              </Text>
            </View>
            <View style={styles.infoRow}>
              <Icon name="eye-off" size={20} color={theme.colors.primary} />
              <Text variant="bodySmall" style={styles.infoText}>
                Zero-knowledge architecture
              </Text>
            </View>
            <View style={styles.infoRow}>
              <Icon name="cloud-off" size={20} color={theme.colors.primary} />
              <Text variant="bodySmall" style={styles.infoText}>
                We never see your passwords
              </Text>
            </View>
          </Card.Content>
        </Card>
      </ScrollView>

      <FAB
        icon="plus"
        style={[styles.fab, { backgroundColor: theme.colors.primary }]}
        onPress={() => {
          // TODO: Navigate to AddItem screen
        }}
      />
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8fafc',
  },
  scrollContent: {
    padding: 16,
    paddingBottom: 100,
  },
  welcomeCard: {
    marginBottom: 16,
    backgroundColor: '#e0f2fe',
  },
  welcomeContent: {
    alignItems: 'center',
    paddingVertical: 24,
  },
  welcomeTitle: {
    marginTop: 16,
    fontWeight: '600',
    textAlign: 'center',
  },
  welcomeText: {
    marginTop: 8,
    color: '#64748b',
    textAlign: 'center',
  },
  emptyCard: {
    marginBottom: 16,
  },
  emptyContent: {
    alignItems: 'center',
    paddingVertical: 48,
  },
  emptyTitle: {
    marginTop: 16,
    fontWeight: '600',
  },
  emptyText: {
    marginTop: 8,
    color: '#64748b',
    textAlign: 'center',
    paddingHorizontal: 16,
  },
  infoCard: {
    backgroundColor: '#f1f5f9',
  },
  infoRow: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 8,
  },
  infoText: {
    marginLeft: 12,
    color: '#475569',
  },
  fab: {
    position: 'absolute',
    right: 16,
    bottom: 16,
  },
});

export default VaultScreen;
