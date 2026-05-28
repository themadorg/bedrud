import { Card, CardContent, CardHeader } from '#/components/ui/card'
import { Input } from '#/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '#/components/ui/tabs'

export function AdminDetailTable() {
  return (
    <Card>
      <CardHeader className="pb-0">
        <Tabs defaultValue="users">
          <div className="flex items-center justify-between">
            <TabsList>
              <TabsTrigger value="users" className="text-xs">
                Users
              </TabsTrigger>
              <TabsTrigger value="rooms" className="text-xs">
                Rooms
              </TabsTrigger>
              <TabsTrigger value="sessions" className="text-xs">
                Sessions
              </TabsTrigger>
            </TabsList>
            <Input placeholder="Search..." className="h-8 w-48 text-xs" />
          </div>
        </Tabs>
      </CardHeader>
      <CardContent className="p-0">
        <Tabs defaultValue="users">
          <TabsContent value="users" className="m-0">
            <div className="p-6 text-center text-xs text-muted-foreground">
              View full user list in the{' '}
              <a href="/dashboard/admin/users" className="text-primary underline underline-offset-2">
                Users
              </a>{' '}
              section.
            </div>
          </TabsContent>
          <TabsContent value="rooms" className="m-0">
            <div className="p-6 text-center text-xs text-muted-foreground">
              View full room list in the{' '}
              <a href="/dashboard/admin/rooms" className="text-primary underline underline-offset-2">
                Rooms
              </a>{' '}
              section.
            </div>
          </TabsContent>
          <TabsContent value="sessions" className="m-0">
            <div className="p-6 text-center text-xs text-muted-foreground">
              Session details available in room participant views.
            </div>
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}
