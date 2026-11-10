<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-2">
      <h3 class="break-words text-lg font-semibold text-gray-900 dark:text-white">#{{ ticket.id }} · {{ ticket.title }}</h3>
      <span class="badge" :class="ticket.status === 'resolved' ? 'badge-success' : 'badge-primary'">{{ t(`feedback.statuses.${ticket.status}`) }}</span>
    </div>
    <div class="flex flex-wrap gap-3 text-sm text-gray-500 dark:text-dark-400">
      <span>{{ t(`feedback.categories.${ticket.category}`) }}</span>
      <span class="text-amber-500" :aria-label="t('feedback.stars', { count: ticket.rating })">{{ '★'.repeat(ticket.rating) }}{{ '☆'.repeat(5 - ticket.rating) }}</span>
      <time>{{ formatDateTime(ticket.created_at) }}</time>
    </div>
    <p class="whitespace-pre-wrap break-words rounded-xl bg-gray-50 p-4 text-sm text-gray-800 dark:bg-dark-800 dark:text-gray-200">{{ ticket.content }}</p>
    <h4 class="font-medium text-gray-900 dark:text-white">{{ t('feedback.replies') }}</h4>
    <p v-if="!ticket.replies?.length" class="text-sm text-gray-500 dark:text-dark-400">{{ t('feedback.noReplies') }}</p>
    <article v-for="reply in ticket.replies" :key="reply.id" class="rounded-xl p-4" :class="reply.author_role === 'user' ? 'bg-gray-100 dark:bg-dark-700' : 'bg-primary-50 dark:bg-primary-900/20'">
      <div class="mb-2 flex flex-wrap items-center justify-between gap-2 text-xs" :class="reply.author_role === 'user' ? 'text-gray-600 dark:text-gray-300' : 'text-primary-700 dark:text-primary-300'">
        <span>{{ t(reply.author_role === 'user' ? 'feedback.userReply' : 'feedback.adminReply') }}</span><time>{{ formatDateTime(reply.created_at) }}</time>
      </div>
      <p class="whitespace-pre-wrap break-words text-sm text-gray-800 dark:text-gray-200">{{ reply.content }}</p>
    </article>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { FeedbackTicket } from '@/api/feedback'
import { formatDateTime } from '@/utils/format'
defineProps<{ ticket: FeedbackTicket }>()
const { t } = useI18n()
</script>
