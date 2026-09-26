# Services

Silicon remains a modular monolith. `agent/` is the one independently deployed process because Docker work must execute on user-selected remote hosts. It exposes typed runtime operations and no unrestricted remote shell.
