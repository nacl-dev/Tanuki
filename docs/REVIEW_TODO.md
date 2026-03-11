# Review TODO

Stand: 2026-03-11

Diese Liste fasst alle Findings aus der Projekt-Review in eine priorisierte, umsetzbare ToDo zusammen.

## Must Fix

- [x] `target_directory` und Schedule-Zielpfade auf erlaubte Roots begrenzen.
  Nur definierte Verzeichnisse wie `/media` und `/downloads` erlauben, keine frei kontrollierten Container-Pfade.

- [x] Plugin-Management auf Admins beschraenken.
  Plugin-Scan, Enable/Disable und Delete duerfen nicht fuer normale User verfuegbar sein.

- [x] SSRF-Schutz fuer Remote-Cover und Poster-Downloads einbauen.
  Private IPs, `localhost`, interne Netze und ungepruefte Redirects blockieren; harte Timeouts und Groessenlimits setzen.

- [x] JWT nicht mehr ueber Asset-URLs als Query-Parameter uebertragen.
  Asset-Auth auf Header, Cookie oder signierte Kurzzeit-URLs umstellen und `?token=` im Backend abschalten.

- [x] `view_count`-Inflation beheben.
  Mutationen wie Rating, Favorite, Metadata-Save, Progress-Save oder Thumbnail-Aktionen duerfen keine Views zaehlen.

- [x] Archive-Support fuer `.cbr` und `.rar` konsistent machen.
  Entweder benoetigte Runtime-Abhaengigkeit wie `unrar` sauber mitliefern oder die Formate offiziell nicht mehr akzeptieren.

- [x] Detailseite auf Routenwechsel reagieren lassen.
  Beim Wechsel von `/media/:id` zu einem anderen Medium muessen Daten, Pages, Duplikate und Collections neu geladen werden.

## Should Fix

- [x] Collection-N+1 im Frontend entfernen.
  Die Library-Seite soll nicht zuerst alle Collections laden und danach jede Collection einzeln per Detail-Request nachziehen.

- [x] Collection-Queries im Backend zusammenziehen.
  Item-Counts, Preview-Daten und Tags nicht pro Collection bzw. pro Item einzeln nachladen.

- [x] Scanner effizienter machen.
  Nicht bei jedem Lauf jede Datei komplett hashen; zuerst `mtime` und `size` pruefen, dann nur bei Aenderungen SHA-256 und pHash neu berechnen.

- [x] Scan, Organize und Auto-Tag als echte Background-Jobs mit Status behandeln.
  Statt synchronen UI-Aktionen lieber Queue/Job-Modell mit Fortschritt, Fehlerstatus und Retry.

- [x] Ownership-Modell klaeren.
  Entscheiden, ob Tanuki eine geteilte Bibliothek oder echte Multi-User-Isolation haben soll, und `owner_id` entsprechend konsequent nutzen oder entfernen.

- [x] Remote-Dateioperationen und Pfadverwendung allgemein haerten.
  Alle usernahen Dateipfade zentral validieren und auf erlaubte Arbeitsbereiche begrenzen.

- [x] Settings-Seite ehrlich machen.
  Entweder echte Persistenz fuer Scan-Interval, Concurrency usw. bauen oder die Werte als Readonly/Systemstatus darstellen.

- [x] Fehlerbehandlung im Frontend professionalisieren.
  `alert()` durch Toasts, Inline-Fehler und klare Success/Error-States ersetzen.

- [x] Hover-Video-Preview optimieren.
  Nicht das Originalvideo fuer Hover-Previews laden, sondern kleine Preview-Artefakte, Clips oder Thumbnail-Sequenzen nutzen.

## Accessibility

- [x] Klickbare `span`-Elemente durch echte Buttons ersetzen.
  Das betrifft unter anderem Rating-Sterne und aehnliche interaktive Elemente.

- [x] Search-Suggestions semantisch und per Tastatur sauber machen.
  `aria-*`, Rollen, aktiven Eintrag, Fokusverhalten und Screenreader-Ausgabe ergaenzen.

- [x] Globale Accessibility-Pruefung fuer Formulare, Modals und Navigation machen.
  Fokusfallen, sichtbare Focus-States, Labels und Tastaturbedienung durchgaengig absichern.

## Design / UX

- [x] Mobile Navigation neu denken.
  Feste Desktop-Sidebar fuer kleine Screens durch Drawer oder Bottom-Navigation ersetzen.

- [x] Visuelle Hierarchie der Library verbessern.
  Suche, Filter, Collections und Grid staerker voneinander absetzen und klarer priorisieren.

- [x] Konsistenteres visuelles System einfuehren.
  Weniger Emoji-UI, dafuer Icon-Set, bewusstere Typografie und staerkere Komponenten-Hierarchie.

- [x] Collections auf der Startseite leichter und wertiger praesentieren.
  Asynchrone Preview-Strips, Cover-Collagen oder dedizierte Preview-Endpoints statt Voll-Laden aller Collection-Inhalte.

- [x] Mehr produktive Library-Werkzeuge einbauen.
  Gespeicherte Ansichten, Dichte-Umschalter, bessere Skeleton-States und "Continue reading" / "Recent" / "Favorites"-Sektionen.

- [x] Settings zu einem echten System-Dashboard ausbauen.
  Runtime-Werte, Worker-Status, Queue-Zahlen, Plugin-Zustand und Health-Daten zentral anzeigen.

## Tooling / Quality

- [x] ESLint korrekt installieren und konfigurieren.
  `npm run lint` muss lokal und in CI lauffaehig sein.

- [x] Go-Tests einfuehren.
  Starten mit Auth, Pfadvalidierung, Collection-Regeln, Download-Target-Validierung und View-Count-Logik.

- [x] Frontend-Tests einfuehren.
  Starten mit Detailseiten-Navigation, Filterzustand, Search-Suggestions und Error-Handling.

- [x] CI fuer Build, Lint und Tests aufsetzen.
  Frontend-Build, Go-Tests, spaeter Frontend-Tests und Lint verpflichtend ausfuehren.

- [x] Vite/esbuild-Toolchain aktualisieren.
  `npm audit` meldet aktuell moderate Findings in der Build-/Dev-Chain.

- [x] Observability verbessern.
  Request-IDs, strukturierte Fehlercodes, bessere Logs und kleine Admin-Health-Ansichten ergaenzen.

## Docs / Config

- [x] `DOWNLOADS_PATH` in README, `.env.example`, `config.go` und `docker-compose.yml` vereinheitlichen.
  Aktuell widersprechen sich Doku und Laufzeitdefaults.

- [x] Tatsaechlich unterstuetzte Archive und Reader-Faehigkeiten dokumentieren.
  Wenn `.rar`/`.cbr` supportet werden sollen, muss das auch im Runtime-Image sichergestellt sein.

- [x] Multi-User-Verhalten dokumentieren.
  Klar festhalten, welche Daten global sind und welche userbezogen sein sollen.

## Optional Reihenfolge

1. Security und Rechte
2. Datenkonsistenz und Reader-Navigation
3. Performance bei Collections und Scanner
4. UX, Accessibility und mobile Layouts
5. Tests, Lint und CI
6. Doku und Produktfeinschliff
