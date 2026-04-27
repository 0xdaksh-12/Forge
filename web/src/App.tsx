import { BrowserRouter, Routes, Route } from "react-router-dom";
import { Sidebar } from "./components/Sidebar";
import { Dashboard } from "./pages/Dashboard";
import { Pipelines } from "./pages/Pipelines";
import { BuildDetail } from "./pages/BuildDetail";

export default function App() {
  return (
    <BrowserRouter>
      <div className="app-shell">
        <Sidebar />
        <main className="main-content">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/pipelines" element={<Pipelines />} />
            <Route path="/pipelines/:id" element={<Pipelines />} />
            <Route path="/builds/:id" element={<BuildDetail />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  );
}
