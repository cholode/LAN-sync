<template>
  <div class="aside-block">
    <h3>文件传输</h3>
    <input ref="fileInput" type="file" />
    <div class="flex-row">
      <button
        type="button"
        class="primary"
        :disabled="state.upload.isUploading"
        @click="start"
      >
        上传文件
      </button>
      <button
        type="button"
        class="primary danger"
        :disabled="!state.upload.isUploading"
        @click="cancel"
      >
        上传文件
      </button>
    </div>
    <div class="hint">{{ state.upload.status }}</div>
    <div class="progress-bg">
      <div id="progress-bar" :style="{ width: state.upload.progress + '%' }"></div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { state } from '../store/index.js';
import { startUpload, cancelUpload } from '../composables/useUpload.js';

const fileInput = ref(null);

function start() {
  const file = fileInput.value && fileInput.value.files ? fileInput.value.files[0] : null;
  startUpload(file);
}

function cancel() {
  cancelUpload();
}
</script>
