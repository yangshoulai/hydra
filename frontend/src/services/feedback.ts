import type { DialogApi, MessageApi, NotificationApi } from 'naive-ui'

interface FeedbackApis {
  message: MessageApi
  dialog: DialogApi
  notification: NotificationApi
}

export const feedback: {
  message: MessageApi | null
  dialog: DialogApi | null
  notification: NotificationApi | null
} = {
  message: null,
  dialog: null,
  notification: null,
}

export function setFeedbackApis(apis: FeedbackApis) {
  feedback.message = apis.message
  feedback.dialog = apis.dialog
  feedback.notification = apis.notification
}
