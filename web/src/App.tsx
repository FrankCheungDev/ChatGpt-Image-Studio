import { Navigate, Route, Routes } from "react-router-dom";

import PlatformHistoryPage from "@/app/admin/history/page";
import ImagePage from "@/app/image/page";
import AppShell from "@/app/layout";
import LoginPage from "@/app/login/page";
import HomePage from "@/app/page";
import UsersPage from "@/app/users/page";

export default function App() {
  return (
    <AppShell>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/image" element={<Navigate to="/image/history" replace />} />
        <Route path="/image/history" element={<ImagePage />} />
        <Route path="/image/workspace" element={<ImagePage />} />
        <Route path="/users" element={<UsersPage />} />
        <Route path="/admin/history" element={<PlatformHistoryPage />} />
        <Route path="*" element={<Navigate to="/image/history" replace />} />
      </Routes>
    </AppShell>
  );
}
