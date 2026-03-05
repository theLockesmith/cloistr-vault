import React, { useState, useEffect, useCallback } from 'react';
import {
  View,
  StyleSheet,
  ScrollView,
  RefreshControl,
  Alert,
} from 'react-native';
import {
  Text,
  Card,
  Button,
  List,
  Divider,
  useTheme,
  IconButton,
  Dialog,
  Portal,
  TextInput,
  ActivityIndicator,
} from 'react-native-paper';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import { useAuth } from '../../contexts/AuthContext';
import { PasskeyCredential } from '../../services/passkey';

const SettingsScreen: React.FC = () => {
  const theme = useTheme();
  const {
    user,
    logout,
    passkeySupported,
    registerPasskey,
    getPasskeys,
    renamePasskey,
    deletePasskey,
    loading,
  } = useAuth();

  const [passkeys, setPasskeys] = useState<PasskeyCredential[]>([]);
  const [refreshing, setRefreshing] = useState(false);
  const [loadingPasskeys, setLoadingPasskeys] = useState(true);

  // Rename dialog state
  const [renameDialogVisible, setRenameDialogVisible] = useState(false);
  const [selectedPasskey, setSelectedPasskey] = useState<PasskeyCredential | null>(null);
  const [newPasskeyName, setNewPasskeyName] = useState('');

  const fetchPasskeys = useCallback(async () => {
    try {
      setLoadingPasskeys(true);
      const credentials = await getPasskeys();
      setPasskeys(credentials);
    } catch (error) {
      // Error handled in context
    } finally {
      setLoadingPasskeys(false);
    }
  }, [getPasskeys]);

  useEffect(() => {
    fetchPasskeys();
  }, [fetchPasskeys]);

  const onRefresh = useCallback(async () => {
    setRefreshing(true);
    await fetchPasskeys();
    setRefreshing(false);
  }, [fetchPasskeys]);

  const handleRegisterPasskey = async () => {
    try {
      await registerPasskey();
      await fetchPasskeys();
    } catch (error) {
      // Error handled in context
    }
  };

  const handleRenamePasskey = (passkey: PasskeyCredential) => {
    setSelectedPasskey(passkey);
    setNewPasskeyName(passkey.name);
    setRenameDialogVisible(true);
  };

  const confirmRename = async () => {
    if (!selectedPasskey || !newPasskeyName.trim()) return;

    try {
      await renamePasskey(selectedPasskey.id, newPasskeyName.trim());
      await fetchPasskeys();
      setRenameDialogVisible(false);
      setSelectedPasskey(null);
      setNewPasskeyName('');
    } catch (error) {
      // Error handled in context
    }
  };

  const handleDeletePasskey = (passkey: PasskeyCredential) => {
    Alert.alert(
      'Remove Passkey',
      `Are you sure you want to remove "${passkey.name}"? You won't be able to use this passkey to sign in anymore.`,
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Remove',
          style: 'destructive',
          onPress: async () => {
            try {
              await deletePasskey(passkey.id);
              await fetchPasskeys();
            } catch (error) {
              // Error handled in context
            }
          },
        },
      ]
    );
  };

  const handleLogout = () => {
    Alert.alert(
      'Sign Out',
      'Are you sure you want to sign out?',
      [
        { text: 'Cancel', style: 'cancel' },
        {
          text: 'Sign Out',
          style: 'destructive',
          onPress: logout,
        },
      ]
    );
  };

  const formatDate = (dateString: string | null) => {
    if (!dateString) return 'Never';
    return new Date(dateString).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  const renderPasskeyItem = (passkey: PasskeyCredential) => (
    <List.Item
      key={passkey.id}
      title={passkey.name}
      description={`Created: ${formatDate(passkey.created_at)} • Last used: ${formatDate(passkey.last_used_at)}`}
      left={(props) => (
        <List.Icon {...props} icon="fingerprint" color={theme.colors.primary} />
      )}
      right={() => (
        <View style={styles.passkeyActions}>
          <IconButton
            icon="pencil"
            size={20}
            onPress={() => handleRenamePasskey(passkey)}
          />
          <IconButton
            icon="delete"
            size={20}
            iconColor={theme.colors.error}
            onPress={() => handleDeletePasskey(passkey)}
          />
        </View>
      )}
      style={styles.passkeyItem}
    />
  );

  return (
    <ScrollView
      style={styles.container}
      contentContainerStyle={styles.contentContainer}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
      }
    >
      {/* Account Section */}
      <Card style={styles.card}>
        <Card.Title
          title="Account"
          left={(props) => <Icon {...props} name="account" size={24} />}
        />
        <Card.Content>
          <List.Item
            title="Email"
            description={user?.email || 'Not available'}
            left={(props) => <List.Icon {...props} icon="email" />}
          />
          <Divider />
          <List.Item
            title="Member Since"
            description={user?.created_at ? formatDate(user.created_at) : 'Unknown'}
            left={(props) => <List.Icon {...props} icon="calendar" />}
          />
        </Card.Content>
      </Card>

      {/* Passkeys Section */}
      <Card style={styles.card}>
        <Card.Title
          title="Passkeys"
          subtitle="Sign in without a password"
          left={(props) => <Icon {...props} name="fingerprint" size={24} />}
        />
        <Card.Content>
          {!passkeySupported ? (
            <View style={styles.notSupported}>
              <Icon name="alert-circle" size={32} color={theme.colors.error} />
              <Text variant="bodyMedium" style={styles.notSupportedText}>
                Passkeys are not supported on this device
              </Text>
              <Text variant="bodySmall" style={styles.notSupportedSubtext}>
                Requires iOS 15+ or Android API 28+
              </Text>
            </View>
          ) : loadingPasskeys ? (
            <View style={styles.loadingContainer}>
              <ActivityIndicator size="small" />
              <Text variant="bodySmall" style={styles.loadingText}>
                Loading passkeys...
              </Text>
            </View>
          ) : (
            <>
              {passkeys.length === 0 ? (
                <View style={styles.emptyState}>
                  <Icon name="key-plus" size={48} color={theme.colors.outline} />
                  <Text variant="bodyMedium" style={styles.emptyStateText}>
                    No passkeys registered
                  </Text>
                  <Text variant="bodySmall" style={styles.emptyStateSubtext}>
                    Add a passkey to sign in with biometrics
                  </Text>
                </View>
              ) : (
                <View style={styles.passkeyList}>
                  {passkeys.map(renderPasskeyItem)}
                </View>
              )}

              <Button
                mode="outlined"
                onPress={handleRegisterPasskey}
                loading={loading}
                disabled={loading}
                icon="plus"
                style={styles.addPasskeyButton}
              >
                Add Passkey
              </Button>
            </>
          )}
        </Card.Content>
      </Card>

      {/* Security Section */}
      <Card style={styles.card}>
        <Card.Title
          title="Security"
          left={(props) => <Icon {...props} name="shield-lock" size={24} />}
        />
        <Card.Content>
          <List.Item
            title="Change Password"
            description="Update your account password"
            left={(props) => <List.Icon {...props} icon="lock-reset" />}
            right={(props) => <List.Icon {...props} icon="chevron-right" />}
            onPress={() => {
              Alert.alert('Coming Soon', 'Password change will be available soon.');
            }}
          />
          <Divider />
          <List.Item
            title="Recovery Codes"
            description="View or regenerate recovery codes"
            left={(props) => <List.Icon {...props} icon="key-chain" />}
            right={(props) => <List.Icon {...props} icon="chevron-right" />}
            onPress={() => {
              Alert.alert('Coming Soon', 'Recovery codes will be available soon.');
            }}
          />
        </Card.Content>
      </Card>

      {/* Sign Out */}
      <Button
        mode="outlined"
        onPress={handleLogout}
        textColor={theme.colors.error}
        style={styles.logoutButton}
        icon="logout"
      >
        Sign Out
      </Button>

      <Text variant="bodySmall" style={styles.versionText}>
        Cloistr Vault v1.0.0
      </Text>

      {/* Rename Dialog */}
      <Portal>
        <Dialog
          visible={renameDialogVisible}
          onDismiss={() => setRenameDialogVisible(false)}
        >
          <Dialog.Title>Rename Passkey</Dialog.Title>
          <Dialog.Content>
            <TextInput
              mode="outlined"
              label="Passkey Name"
              value={newPasskeyName}
              onChangeText={setNewPasskeyName}
              autoFocus
            />
          </Dialog.Content>
          <Dialog.Actions>
            <Button onPress={() => setRenameDialogVisible(false)}>Cancel</Button>
            <Button onPress={confirmRename} disabled={!newPasskeyName.trim()}>
              Rename
            </Button>
          </Dialog.Actions>
        </Dialog>
      </Portal>
    </ScrollView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8fafc',
  },
  contentContainer: {
    padding: 16,
    paddingBottom: 32,
  },
  card: {
    marginBottom: 16,
  },
  passkeyList: {
    marginBottom: 16,
  },
  passkeyItem: {
    paddingVertical: 8,
  },
  passkeyActions: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  addPasskeyButton: {
    marginTop: 8,
  },
  notSupported: {
    alignItems: 'center',
    paddingVertical: 24,
  },
  notSupportedText: {
    marginTop: 12,
    color: '#64748b',
  },
  notSupportedSubtext: {
    marginTop: 4,
    color: '#94a3b8',
  },
  loadingContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: 24,
  },
  loadingText: {
    marginLeft: 12,
    color: '#64748b',
  },
  emptyState: {
    alignItems: 'center',
    paddingVertical: 24,
  },
  emptyStateText: {
    marginTop: 12,
    color: '#64748b',
  },
  emptyStateSubtext: {
    marginTop: 4,
    color: '#94a3b8',
  },
  logoutButton: {
    marginTop: 8,
    borderColor: '#ef4444',
  },
  versionText: {
    textAlign: 'center',
    color: '#94a3b8',
    marginTop: 24,
  },
});

export default SettingsScreen;
