import "./globals.css"
import NavBar from "@/components/NavBar"

export const metadata = { title: "Swearjar" }

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark">
      <body className="antialiased">
        <NavBar />
        <div className="pt-16 md:pt-20">{children}</div>
      </body>
    </html>
  )
}
