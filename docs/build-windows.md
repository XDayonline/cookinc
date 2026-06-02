# Building cookinc on Windows

Guide pour compiler et développer cookinc sur Windows.

## Prérequis

### 1. Installer Go

```powershell
winget install GoLang.Go
```

Vérifier :
```powershell
go version
# → go version go1.24+ windows/amd64
```

### 2. Installer Git

```powershell
winget install Git.Git
```

### 3. (Optionnel) Scoop

Scoop facilite la gestion des outils dev :

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
irm get.scoop.sh | iex
scoop install curl
```

## Cloner et compiler

```powershell
# Cloner
git clone https://github.com/XDayonline/cookinc.git
cd cookinc

# Télécharger les dépendances
go mod tidy

# Compiler le daemon Windows
go build -o cookinc.exe ./cmd/cookinc/

# Vérifier
.\cookinc.exe
# → cookinc — Windows daemon (not yet implemented)
```

## Structure des dossiers sur Windows

```
%USERPROFILE%\.config\cookinc\       ← config
%USERPROFILE%\.cookinc\              ← cache, cookies.db
```

Le chemin par défaut du Chrome Cookies SQLite :
```
%LOCALAPPDATA%\Google\Chrome\User Data\Default\Network\Cookies
```

## Build cross-platform (depuis Windows)

Pour build le binaire Linux aussi :

```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"
go build -o cookinc-mcp ./cmd/cookinc-mcp/
```

Ou utiliser GoReleaser directement :

```powershell
# Installer goreleaser
go install github.com/goreleaser/goreleaser/v2@latest

# Build tout
goreleaser build --snapshot --clean
```

## Débogage

### Lire le Chrome Cookies SQLite

```powershell
# Le fichier est verrouillé par Chrome quand il tourne
# Il faut d'abord fermer Chrome, ou lire une copie
copy "$env:LOCALAPPDATA\Google\Chrome\User Data\Default\Network\Cookies" .\cookies_backup
```

### Variables d'env utiles

```powershell
$env:COOKINC_DEBUG=1          # logs verbeux
$env:COOKINC_CONFIG_DIR="C:\cookinc"  # config custom
```

## Problèmes connus

| Problème | Solution |
|----------|----------|
| Chrome verrouille le fichier Cookies | Lister les handles avec `Handle.exe` ou fermer Chrome |
| DPAPI ne marche pas en remote | DPAPI est lié à la session utilisateur Windows. Le daemon doit tourner dans la même session que Chrome. |
| Firewall bloque | Autoriser cookinc.exe dans le firewall Windows |
