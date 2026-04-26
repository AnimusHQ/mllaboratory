export type ErrorDescriptor = {
  title: string;
  hint: string;
};

export function describeError(code: string | undefined): ErrorDescriptor {
  const normalized = (code ?? '').toLowerCase();
  switch (normalized) {
    case 'validation_failed':
      return {
        title: 'Ошибки валидации запроса',
        hint: 'Проверьте обязательные поля и форматы значений. Уточните недостающие параметры.',
      };
    case 'precondition_failed':
      return {
        title: 'Не выполнены предварительные условия',
        hint: 'Проверьте наличие артефактов, блокировок окружения и указанных commit pin.',
      };
    case 'quota_exceeded':
      return {
        title: 'Квота или лимит конкурентности превышены',
        hint: 'Уточните лимиты проекта и состояние очереди. Повторите попытку после освобождения ресурсов.',
      };
    case 'upstream_unavailable':
      return {
        title: 'Недоступен upstream сервис',
        hint: 'Повторите запрос позже или сверьте статус доступности в разделе Ops.',
      };
    case 'project_id_required':
      return {
        title: 'Не выбран активный проект',
        hint: 'Выберите или создайте проект. Контекст нужен для выполнения операции.',
      };
    default:
      return {
        title: 'Сбой выполнения операции',
        hint: 'Проверьте корректность входных параметров и повторите запрос. При необходимости приложите диагностический пакет.',
      };
  }
}
