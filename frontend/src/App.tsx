import { Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/session";
import { Layout } from "./components/Layout";
import { RequireAuth } from "./components/RequireAuth";
import AppointmentRequest from "./pages/AppointmentRequest";
import Home from "./pages/Home";
import LegalNotice from "./pages/LegalNotice";
import OrderStatus from "./pages/OrderStatus";
import PrivacyPolicy from "./pages/PrivacyPolicy";
import WorkshopDashboard from "./pages/WorkshopDashboard";
import WorkshopLogin from "./pages/WorkshopLogin";
import WorkshopOrderDetail from "./pages/WorkshopOrderDetail";
import WorkshopOrders from "./pages/WorkshopOrders";

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Home />} />
          <Route path="/terminanfrage" element={<AppointmentRequest />} />
          <Route path="/auftragsstatus" element={<OrderStatus />} />
          <Route path="/impressum" element={<LegalNotice />} />
          <Route path="/datenschutz" element={<PrivacyPolicy />} />
          <Route path="/werkstatt/anmeldung" element={<WorkshopLogin />} />
          <Route
            path="/werkstatt/auftraege"
            element={
              <RequireAuth>
                <WorkshopOrders />
              </RequireAuth>
            }
          />
          <Route
            path="/werkstatt/auftraege/:orderNumber"
            element={
              <RequireAuth>
                <WorkshopOrderDetail />
              </RequireAuth>
            }
          />
          <Route
            path="/werkstatt/dashboard"
            element={
              <RequireAuth>
                <WorkshopDashboard />
              </RequireAuth>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
