<template>
  <LoginView v-if="!isLoggedIn" />
  <ChatView v-else />
</template>

<script setup>
import { computed } from 'vue';
import { state } from './store/index.js';
import LoginView from './components/LoginView.vue';
import ChatView from './components/ChatView.vue';

const isLoggedIn = computed(() => Boolean(state.jwtToken));
</script>
