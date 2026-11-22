<template>
  <div
    class="context-menu"
    ref="contextMenu"
    v-show="show"
    :style="{
      top: `${top}px`,
      left: `${left}px`,
    }"
  >
    <slot />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, onUnmounted } from "vue";

const emit = defineEmits(["hide"]);
const props = defineProps<{ show: boolean; pos: { x: number; y: number } }>();
const contextMenu = ref<HTMLElement | null>(null);

const left = computed(() => {
  const w = contextMenu.value?.offsetWidth || 200;
  return Math.max(8, Math.min(props.pos.x, window.innerWidth - w - 8));
});

const top = computed(() => {
  const h = contextMenu.value?.offsetHeight || 260;
  return Math.max(8, Math.min(props.pos.y, window.innerHeight - h - 8));
});

const hideContextMenu = () => {
  emit("hide");
};

watch(
  () => props.show,
  (val) => {
    if (val) {
      document.addEventListener("click", hideContextMenu);
      document.addEventListener("scroll", hideContextMenu, true);
    } else {
      document.removeEventListener("click", hideContextMenu);
      document.removeEventListener("scroll", hideContextMenu, true);
    }
  }
);

onUnmounted(() => {
  document.removeEventListener("click", hideContextMenu);
  document.removeEventListener("scroll", hideContextMenu, true);
});
</script>
