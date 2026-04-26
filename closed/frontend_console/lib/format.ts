export const formatDateTime = (value?: string | null) => {
  if (!value) {
    return '—';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toISOString().replace('T', ' ').replace('Z', ' UTC');
};

export const formatDurationSeconds = (seconds?: number | null) => {
  if (seconds === null || seconds === undefined) {
    return '—';
  }
  const total = Math.max(0, Math.floor(seconds));
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return `${mins}м ${secs}с`;
};

export const formatBoolean = (value?: boolean | null) => {
  if (value === null || value === undefined) {
    return '—';
  }
  return value ? 'да' : 'нет';
};
