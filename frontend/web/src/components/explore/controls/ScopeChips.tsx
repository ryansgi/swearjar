"use client"
import { useExploreParams } from "@/components/hooks/useExploreParams"
import { Button } from "@/components/ui/button"

function Chip({ label, onRemove }: { label: string; onRemove: () => void }) {
  return (
    <span className="bg-muted inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs">
      {label}
      <button onClick={onRemove} className="hover:opacity-80">
        ×
      </button>
    </span>
  )
}
export function ScopeChips() {
  const { repo, actor, lang, removeScope, addScope } = useExploreParams()
  const chips = [
    ...repo
      .split(",")
      .filter(Boolean)
      .map((v) => ({ k: "repo" as const, v })),
    ...actor
      .split(",")
      .filter(Boolean)
      .map((v) => ({ k: "actor" as const, v })),
    ...lang
      .split(",")
      .filter(Boolean)
      .map((v) => ({ k: "lang" as const, v })),
  ]
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="text-muted-foreground text-sm">Scope:</span>
      {chips.length === 0 && <span className="text-muted-foreground text-sm">(none)</span>}
      {chips.map(({ k, v }) => (
        <Chip key={`${k}:${v}`} label={`${k}:${v}`} onRemove={() => removeScope(k, v)} />
      ))}
      <div className="flex items-center gap-2">
        <Button variant="outline" size="sm" onClick={() => addScope("repo", "hid123")}>
          + Repo HID
        </Button>
        <Button variant="outline" size="sm" onClick={() => addScope("actor", "hidA")}>
          + Actor HID
        </Button>
        <Button variant="outline" size="sm" onClick={() => addScope("lang", "en")}>
          + Language
        </Button>
      </div>
    </div>
  )
}
