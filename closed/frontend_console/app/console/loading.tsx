import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

export default function ConsoleLoading() {
  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle>Загрузка контура</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-sm text-muted-foreground">Ожидается ответ Gateway API. Все операции остаются детерминированными.</div>
        </CardContent>
      </Card>
    </div>
  );
}
