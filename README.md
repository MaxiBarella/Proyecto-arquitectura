# Proyecto arquitectura

Proyecto de la materia Arquitectura de Software, trabajado con [BMAD Method](https://github.com/bmad-code-org/BMAD-METHOD) (v6.12.0).

## Estructura

- `_bmad/` — instalación de BMAD (módulos `core` y `bmm`). No se edita a mano; las personalizaciones del equipo van en `_bmad/custom/`.
- `.claude/skills/` — skills de BMAD para Claude Code.
- `_bmad-output/` — artefactos que generan los agentes (brief, PRD, arquitectura, historias).
- `docs/` — conocimiento del proyecto que los agentes toman como contexto. El punto de partida es [docs/enunciado-tp-integrador.md](docs/enunciado-tp-integrador.md), el enunciado del práctico analizado.

Las reglas para los agentes de IA están en [AGENTS.md](AGENTS.md): toda decisión la toma el equipo.

## Cómo empezar

1. Clonar el repo y abrir la carpeta con Claude Code.
2. Invocar la skill `bmad-help` y preguntarle por dónde seguir.

Para actualizar BMAD:

```bash
npx bmad-method@latest install
```

## Forma de trabajo

`main` es la rama común. Cada integrante trabaja en su propia rama y abre un pull request hacia `main`.
