export function AdminSettings() {
  return (
    <div className="space-y-6">
      <div className="sm:flex sm:items-center sm:justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Настройки</h1>
      </div>

      <div className="bg-white shadow overflow-hidden sm:rounded-lg">
        <div className="px-4 py-5 sm:px-6">
          <h3 className="text-lg leading-6 font-medium text-gray-900">Admin Profile</h3>
          <p className="mt-1 max-w-2xl text-sm text-gray-500">Personal information and preferences.</p>
        </div>
        <div className="border-t border-gray-200 px-4 py-5 sm:p-0">
          <dl className="sm:divide-y sm:divide-gray-200">
            <div className="py-4 sm:py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
              <dt className="text-sm font-medium text-gray-500">Full name</dt>
              <dd className="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">Admin User</dd>
            </div>
            <div className="py-4 sm:py-5 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">
              <dt className="text-sm font-medium text-gray-500">Email address</dt>
              <dd className="mt-1 text-sm text-gray-900 sm:mt-0 sm:col-span-2">admin@zamk.com</dd>
            </div>
          </dl>
        </div>
      </div>

      <div className="bg-white shadow overflow-hidden sm:rounded-lg">
        <div className="px-4 py-5 sm:px-6">
          <h3 className="text-lg leading-6 font-medium text-gray-900">System Settings</h3>
          <p className="mt-1 max-w-2xl text-sm text-gray-500">Global configurations for ZAMK platform.</p>
        </div>
        <div className="border-t border-gray-200 p-6">
          <p className="text-sm text-gray-500">Settings form placeholders...</p>
        </div>
      </div>
      
      <div className="bg-white shadow overflow-hidden sm:rounded-lg">
        <div className="px-4 py-5 sm:px-6">
          <h3 className="text-lg leading-6 font-medium text-gray-900">Security & Notifications</h3>
          <p className="mt-1 max-w-2xl text-sm text-gray-500">Manage 2FA and notification rules.</p>
        </div>
        <div className="border-t border-gray-200 p-6">
          <p className="text-sm text-gray-500">Security configurations placeholders...</p>
        </div>
      </div>
    </div>
  );
}
