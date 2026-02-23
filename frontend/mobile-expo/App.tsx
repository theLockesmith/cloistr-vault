import React, { useState, useEffect } from 'react';
import { StyleSheet, Text, View, TextInput, TouchableOpacity, ScrollView, Alert } from 'react-native';
import { StatusBar } from 'expo-status-bar';

interface VaultEntry {
  id: string;
  name: string;
  username: string;
  password: string;
  url: string;
}

export default function App() {
  const [isUnlocked, setIsUnlocked] = useState(false);
  const [masterPassword, setMasterPassword] = useState('');
  const [vault, setVault] = useState<VaultEntry[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  
  // New entry form
  const [showAddForm, setShowAddForm] = useState(false);
  const [newEntry, setNewEntry] = useState({
    name: '',
    username: '',
    password: '',
    url: ''
  });

  const unlockVault = async () => {
    if (masterPassword === 'demo123') {
      const demoVault = [
        {
          id: '1',
          name: 'Welcome to Cloistr Vault',
          username: 'demo@example.com',
          password: 'SecurePassword123!',
          url: 'https://cloistr.com'
        },
        {
          id: '2', 
          name: 'GitHub',
          username: 'your_username',
          password: 'SuperSecret456!',
          url: 'https://github.com'
        }
      ];
      
      setVault(demoVault);
      setIsUnlocked(true);
      Alert.alert('Success', 'Vault unlocked! 🔓');
    } else {
      Alert.alert('Error', 'Invalid master password. Try: demo123');
    }
  };

  const lockVault = () => {
    setIsUnlocked(false);
    setVault([]);
    setMasterPassword('');
    Alert.alert('Locked', 'Vault has been locked 🔒');
  };

  const addEntry = () => {
    if (!newEntry.name || !newEntry.password) {
      Alert.alert('Error', 'Name and password are required');
      return;
    }

    const entry: VaultEntry = {
      id: Date.now().toString(),
      ...newEntry
    };

    setVault([...vault, entry]);
    setNewEntry({ name: '', username: '', password: '', url: '' });
    setShowAddForm(false);
    
    Alert.alert('Success', 'Entry added to vault! 🔐');
  };

  const copyToClipboard = (text: string, label: string) => {
    Alert.alert('Copied', `${label} copied to clipboard! 📋`);
  };

  if (!isUnlocked) {
    return (
      <View style={styles.container}>
        <StatusBar style="light" />
        <View style={styles.unlockContainer}>
          <Text style={styles.logo}>🛡️</Text>
          <Text style={styles.title}>Cloistr Vault</Text>
          <Text style={styles.subtitle}>Zero-knowledge password manager</Text>
          
          <View style={styles.unlockForm}>
            <Text style={styles.label}>Master Password</Text>
            <TextInput
              style={styles.input}
              placeholder="Enter master password"
              value={masterPassword}
              onChangeText={setMasterPassword}
              secureTextEntry
              autoCapitalize="none"
            />
            
            <TouchableOpacity style={styles.button} onPress={unlockVault}>
              <Text style={styles.buttonText}>Unlock Vault</Text>
            </TouchableOpacity>
          </View>
          
          <Text style={styles.demoText}>
            Demo password: <Text style={styles.demoPassword}>demo123</Text>
          </Text>
        </View>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <StatusBar style="dark" />
      <View style={styles.header}>
        <Text style={styles.headerTitle}>🛡️ Your Vault ({vault.length} items)</Text>
        <TouchableOpacity style={styles.lockButton} onPress={lockVault}>
          <Text style={styles.lockButtonText}>🔒</Text>
        </TouchableOpacity>
      </View>

      <ScrollView style={styles.content}>
        {vault.map((entry) => (
          <View key={entry.id} style={styles.entryCard}>
            <View style={styles.entryHeader}>
              <Text style={styles.entryName}>🌐 {entry.name}</Text>
            </View>
            
            <TouchableOpacity 
              style={styles.entryField}
              onPress={() => copyToClipboard(entry.username, 'Username')}
            >
              <Text style={styles.fieldLabel}>Username:</Text>
              <Text style={styles.fieldValue}>{entry.username}</Text>
            </TouchableOpacity>
            
            <TouchableOpacity 
              style={styles.entryField}
              onPress={() => copyToClipboard(entry.password, 'Password')}
            >
              <Text style={styles.fieldLabel}>Password:</Text>
              <Text style={styles.fieldValue}>••••••••••••</Text>
            </TouchableOpacity>
            
            {entry.url && (
              <View style={styles.entryField}>
                <Text style={styles.fieldLabel}>URL:</Text>
                <Text style={styles.fieldValue}>{entry.url}</Text>
              </View>
            )}
          </View>
        ))}

        {showAddForm && (
          <View style={styles.addForm}>
            <Text style={styles.addFormTitle}>Add New Entry</Text>
            
            <TextInput
              style={styles.input}
              placeholder="Service name"
              value={newEntry.name}
              onChangeText={(text) => setNewEntry({...newEntry, name: text})}
            />
            
            <TextInput
              style={styles.input}
              placeholder="Username/Email"
              value={newEntry.username}
              onChangeText={(text) => setNewEntry({...newEntry, username: text})}
              autoCapitalize="none"
            />
            
            <TextInput
              style={styles.input}
              placeholder="Password"
              value={newEntry.password}
              onChangeText={(text) => setNewEntry({...newEntry, password: text})}
              secureTextEntry
            />
            
            <TextInput
              style={styles.input}
              placeholder="Website URL (optional)"
              value={newEntry.url}
              onChangeText={(text) => setNewEntry({...newEntry, url: text})}
              autoCapitalize="none"
            />
            
            <View style={styles.formButtons}>
              <TouchableOpacity style={styles.cancelButton} onPress={() => setShowAddForm(false)}>
                <Text style={styles.cancelButtonText}>Cancel</Text>
              </TouchableOpacity>
              
              <TouchableOpacity style={styles.button} onPress={addEntry}>
                <Text style={styles.buttonText}>Add Entry</Text>
              </TouchableOpacity>
            </View>
          </View>
        )}
      </ScrollView>

      {!showAddForm && (
        <TouchableOpacity 
          style={styles.fab} 
          onPress={() => setShowAddForm(true)}
        >
          <Text style={styles.fabText}>+</Text>
        </TouchableOpacity>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f8fafc',
  },
  unlockContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    paddingHorizontal: 32,
  },
  logo: {
    fontSize: 64,
    marginBottom: 16,
  },
  title: {
    fontSize: 28,
    fontWeight: '600',
    color: '#1e293b',
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 16,
    color: '#64748b',
    marginBottom: 48,
    textAlign: 'center',
  },
  unlockForm: {
    width: '100%',
    maxWidth: 300,
  },
  label: {
    fontSize: 16,
    fontWeight: '500',
    color: '#374151',
    marginBottom: 8,
  },
  input: {
    borderWidth: 1,
    borderColor: '#d1d5db',
    borderRadius: 8,
    padding: 12,
    fontSize: 16,
    backgroundColor: '#ffffff',
    marginBottom: 16,
  },
  button: {
    backgroundColor: '#2563eb',
    borderRadius: 8,
    padding: 16,
    alignItems: 'center',
    flex: 0.45,
  },
  buttonText: {
    color: '#ffffff',
    fontSize: 16,
    fontWeight: '600',
  },
  demoText: {
    marginTop: 24,
    fontSize: 14,
    color: '#64748b',
  },
  demoPassword: {
    fontFamily: 'monospace',
    backgroundColor: '#e2e8f0',
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 4,
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingHorizontal: 16,
    paddingVertical: 16,
    paddingTop: 48,
    backgroundColor: '#ffffff',
    borderBottomWidth: 1,
    borderBottomColor: '#e2e8f0',
  },
  headerTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#1e293b',
  },
  lockButton: {
    backgroundColor: '#ef4444',
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 6,
  },
  lockButtonText: {
    color: '#ffffff',
    fontSize: 24,
  },
  content: {
    flex: 1,
    paddingHorizontal: 16,
    paddingTop: 16,
  },
  entryCard: {
    backgroundColor: '#ffffff',
    borderRadius: 12,
    padding: 16,
    marginBottom: 12,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  entryHeader: {
    marginBottom: 12,
  },
  entryName: {
    fontSize: 18,
    fontWeight: '600',
    color: '#1e293b',
  },
  entryField: {
    marginBottom: 8,
  },
  fieldLabel: {
    fontSize: 12,
    fontWeight: '500',
    color: '#64748b',
    marginBottom: 2,
  },
  fieldValue: {
    fontSize: 14,
    color: '#374151',
    padding: 8,
    backgroundColor: '#f8fafc',
    borderRadius: 6,
  },
  addForm: {
    backgroundColor: '#ffffff',
    borderRadius: 12,
    padding: 16,
    marginBottom: 16,
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 3,
  },
  addFormTitle: {
    fontSize: 18,
    fontWeight: '600',
    color: '#1e293b',
    marginBottom: 16,
  },
  formButtons: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginTop: 8,
  },
  cancelButton: {
    backgroundColor: '#6b7280',
    borderRadius: 8,
    padding: 12,
    flex: 0.45,
    alignItems: 'center',
  },
  cancelButtonText: {
    color: '#ffffff',
    fontSize: 14,
    fontWeight: '600',
  },
  fab: {
    position: 'absolute',
    bottom: 24,
    right: 24,
    width: 56,
    height: 56,
    borderRadius: 28,
    backgroundColor: '#2563eb',
    justifyContent: 'center',
    alignItems: 'center',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 4 },
    shadowOpacity: 0.3,
    shadowRadius: 8,
    elevation: 8,
  },
  fabText: {
    color: '#ffffff',
    fontSize: 24,
    fontWeight: '300',
  },
});
