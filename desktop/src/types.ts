export interface Project {
  id: string;
  name: string;
  path: string;
}
export interface Member {
  id: string;
  name: string;
  role: string;
}
export interface Goal {
  id: string;
  project_id: string;
  title: string;
  description: string;
  state: "awaiting_engine";
  version: number;
}
export interface Snapshot {
  protocol_version: number;
  projects: Project[];
  members: Member[];
  goals: Goal[];
  cursor: number;
}
export interface ProjectInput {
  request_id: string;
  name: string;
  path: string;
}
export interface GoalInput {
  request_id: string;
  project_id: string;
  title: string;
  description: string;
}

declare global {
  interface Window {
    team: {
      snapshot(): Promise<Snapshot>;
      createProject(input: ProjectInput): Promise<Project>;
      createGoal(input: GoalInput): Promise<Goal>;
      selectDirectory(): Promise<string | null>;
    };
  }
}
