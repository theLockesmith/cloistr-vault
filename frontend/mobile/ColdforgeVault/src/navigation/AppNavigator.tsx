import React from 'react';
import { createStackNavigator } from '@react-navigation/stack';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { View, Text, StyleSheet } from 'react-native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import { useAuth } from '../contexts/AuthContext';
import LoginScreen from '../screens/auth/LoginScreen';
import VaultScreen from '../screens/main/VaultScreen';
import SettingsScreen from '../screens/main/SettingsScreen';
import LoadingScreen from '../components/LoadingScreen';

export type RootStackParamList = {
  Auth: undefined;
  Main: undefined;
  AddItem: undefined;
  ItemDetail: { itemId: string };
};

export type AuthStackParamList = {
  Login: undefined;
  Register: undefined;
};

export type MainTabParamList = {
  Vault: undefined;
  Settings: undefined;
};

const RootStack = createStackNavigator<RootStackParamList>();
const AuthStack = createStackNavigator<AuthStackParamList>();
const MainTab = createBottomTabNavigator<MainTabParamList>();

// Placeholder screen for Register
function RegisterScreen() {
  return (
    <View style={styles.placeholder}>
      <Icon name="account-plus" size={64} color="#94a3b8" />
      <Text style={styles.placeholderText}>Registration Coming Soon</Text>
    </View>
  );
}

// Placeholder screen for AddItem
function AddItemScreen() {
  return (
    <View style={styles.placeholder}>
      <Icon name="plus-circle" size={64} color="#94a3b8" />
      <Text style={styles.placeholderText}>Add Item Coming Soon</Text>
    </View>
  );
}

// Placeholder screen for ItemDetail
function ItemDetailScreen() {
  return (
    <View style={styles.placeholder}>
      <Icon name="file-document" size={64} color="#94a3b8" />
      <Text style={styles.placeholderText}>Item Details Coming Soon</Text>
    </View>
  );
}

function AuthNavigator() {
  return (
    <AuthStack.Navigator
      screenOptions={{
        headerShown: false,
        cardStyle: { backgroundColor: '#f8fafc' },
      }}
    >
      <AuthStack.Screen name="Login" component={LoginScreen} />
      <AuthStack.Screen name="Register" component={RegisterScreen} />
    </AuthStack.Navigator>
  );
}

function MainNavigator() {
  return (
    <MainTab.Navigator
      screenOptions={({ route }) => ({
        tabBarIcon: ({ focused, color, size }) => {
          let iconName: string;

          if (route.name === 'Vault') {
            iconName = focused ? 'shield-key' : 'shield-key-outline';
          } else if (route.name === 'Settings') {
            iconName = focused ? 'cog' : 'cog-outline';
          } else {
            iconName = 'help-circle';
          }

          return <Icon name={iconName} size={size} color={color} />;
        },
        tabBarActiveTintColor: '#2563eb',
        tabBarInactiveTintColor: '#64748b',
        tabBarStyle: {
          backgroundColor: '#ffffff',
          borderTopColor: '#e2e8f0',
          paddingBottom: 8,
          paddingTop: 8,
          height: 60,
        },
        headerStyle: {
          backgroundColor: '#2563eb',
        },
        headerTintColor: '#ffffff',
        headerTitleStyle: {
          fontWeight: '600',
        },
      })}
    >
      <MainTab.Screen
        name="Vault"
        component={VaultScreen}
        options={{ title: 'My Vault' }}
      />
      <MainTab.Screen
        name="Settings"
        component={SettingsScreen}
        options={{ title: 'Settings' }}
      />
    </MainTab.Navigator>
  );
}

export default function AppNavigator() {
  const { user, loading } = useAuth();

  if (loading) {
    return <LoadingScreen />;
  }

  return (
    <RootStack.Navigator screenOptions={{ headerShown: false }}>
      {user ? (
        <>
          <RootStack.Screen name="Main" component={MainNavigator} />
          <RootStack.Screen
            name="AddItem"
            component={AddItemScreen}
            options={{ presentation: 'modal' }}
          />
          <RootStack.Screen
            name="ItemDetail"
            component={ItemDetailScreen}
            options={{ presentation: 'card' }}
          />
        </>
      ) : (
        <RootStack.Screen name="Auth" component={AuthNavigator} />
      )}
    </RootStack.Navigator>
  );
}

const styles = StyleSheet.create({
  placeholder: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: '#f8fafc',
  },
  placeholderText: {
    marginTop: 16,
    fontSize: 18,
    color: '#64748b',
  },
});
