//go:build windows

package setup

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/overseer/overseer/cmd/desktop/internal/config"
	"github.com/overseer/overseer/cmd/desktop/internal/locale"
)

// ShowSettingsDialog displays a polished settings dialog on Windows.
func ShowSettingsDialog(cfg *config.Config) (newServerURL string, reRegister bool, err error) {
	ps := fmt.Sprintf(`
Add-Type -AssemblyName PresentationFramework
Add-Type -AssemblyName PresentationCore

[xml]$xaml = @"
<Window xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
        xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml"
        Title="%s" Height="360" Width="460"
        WindowStartupLocation="CenterScreen" ResizeMode="NoResize"
        Background="#F9FAFB">
    <Window.Resources>
        <Style TargetType="TextBlock" x:Key="Label">
            <Setter Property="FontSize" Value="12"/>
            <Setter Property="Foreground" Value="#374151"/>
            <Setter Property="FontWeight" Value="SemiBold"/>
            <Setter Property="Margin" Value="0,0,0,4"/>
        </Style>
        <Style TargetType="TextBox">
            <Setter Property="Padding" Value="8,6"/>
            <Setter Property="FontSize" Value="13"/>
            <Setter Property="BorderBrush" Value="#D1D5DB"/>
            <Setter Property="BorderThickness" Value="1"/>
        </Style>
        <Style TargetType="TextBlock" x:Key="Info">
            <Setter Property="FontSize" Value="12"/>
            <Setter Property="Foreground" Value="#6B7280"/>
            <Setter Property="Padding" Value="8,6"/>
        </Style>
        <Style TargetType="Button" x:Key="Primary">
            <Setter Property="Background" Value="#4F46E5"/>
            <Setter Property="Foreground" Value="White"/>
            <Setter Property="FontSize" Value="12"/>
            <Setter Property="Padding" Value="16,6"/>
            <Setter Property="BorderThickness" Value="0"/>
            <Setter Property="Cursor" Value="Hand"/>
        </Style>
        <Style TargetType="Button" x:Key="Secondary">
            <Setter Property="Background" Value="White"/>
            <Setter Property="Foreground" Value="#374151"/>
            <Setter Property="FontSize" Value="12"/>
            <Setter Property="Padding" Value="16,6"/>
            <Setter Property="BorderBrush" Value="#D1D5DB"/>
            <Setter Property="BorderThickness" Value="1"/>
            <Setter Property="Cursor" Value="Hand"/>
        </Style>
        <Style TargetType="Button" x:Key="Danger">
            <Setter Property="Background" Value="#EF4444"/>
            <Setter Property="Foreground" Value="White"/>
            <Setter Property="FontSize" Value="12"/>
            <Setter Property="Padding" Value="16,6"/>
            <Setter Property="BorderThickness" Value="0"/>
            <Setter Property="Cursor" Value="Hand"/>
        </Style>
    </Window.Resources>
    <Border Margin="24" CornerRadius="8" Background="White" BorderBrush="#E5E7EB" BorderThickness="1" Padding="24">
        <Grid>
            <Grid.RowDefinitions>
                <RowDefinition Height="Auto"/>
                <RowDefinition Height="20"/>
                <RowDefinition Height="Auto"/>
                <RowDefinition Height="Auto"/>
                <RowDefinition Height="16"/>
                <RowDefinition Height="Auto"/>
                <RowDefinition Height="Auto"/>
                <RowDefinition Height="16"/>
                <RowDefinition Height="Auto"/>
                <RowDefinition Height="Auto"/>
                <RowDefinition Height="*"/>
                <RowDefinition Height="Auto"/>
            </Grid.RowDefinitions>

            <TextBlock Grid.Row="0" FontSize="16" FontWeight="Bold" Foreground="#111827" Text="⚙ %s"/>

            <TextBlock Grid.Row="2" Style="{StaticResource Label}" Text="%s"/>
            <TextBox Grid.Row="3" Name="ServerURL" Text="%s"/>

            <TextBlock Grid.Row="5" Style="{StaticResource Label}" Text="%s"/>
            <TextBlock Grid.Row="6" Style="{StaticResource Info}" Text="%s" Background="#F3F4F6"/>

            <TextBlock Grid.Row="8" Style="{StaticResource Label}" Text="%s"/>
            <TextBlock Grid.Row="9" Style="{StaticResource Info}" Text="%s" Background="#F3F4F6" FontFamily="Consolas" FontSize="11"/>

            <StackPanel Grid.Row="11" Orientation="Horizontal" HorizontalAlignment="Right">
                <Button Name="BtnReRegister" Content="%s" Style="{StaticResource Danger}" Margin="0,0,8,0"/>
                <Button Name="BtnCancel" Content="%s" Style="{StaticResource Secondary}" Margin="0,0,8,0"/>
                <Button Name="BtnSave" Content="%s" Style="{StaticResource Primary}" IsDefault="True"/>
            </StackPanel>
        </Grid>
    </Border>
</Window>
"@

$reader = (New-Object System.Xml.XmlNodeReader $xaml)
$window = [Windows.Markup.XamlReader]::Load($reader)

$serverURL = $window.FindName("ServerURL")
$btnSave = $window.FindName("BtnSave")
$btnCancel = $window.FindName("BtnCancel")
$btnReRegister = $window.FindName("BtnReRegister")

$script:result = ""

$btnSave.Add_Click({
    $script:result = "save|$($serverURL.Text)"
    $window.Close()
})

$btnCancel.Add_Click({
    $script:result = ""
    $window.Close()
})

$btnReRegister.Add_Click({
    $script:result = "reregister|$($serverURL.Text)"
    $window.Close()
})

$window.ShowDialog() | Out-Null
Write-Output $script:result
`,
		locale.T("settings.title"),
		locale.T("settings.title"),
		locale.T("settings.server_url"),
		cfg.ServerURL,
		locale.T("settings.device_name"),
		cfg.DeviceName,
		locale.T("settings.device_id"),
		cfg.DeviceID,
		locale.T("settings.re_register"),
		locale.T("settings.cancel"),
		locale.T("settings.save"),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-STA", "-Command", ps)
	cmd.SysProcAttr = hiddenProcAttr()
	output, err := cmd.Output()
	if err != nil {
		return "", false, fmt.Errorf("settings dialog error: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", false, nil
	}

	parts := strings.SplitN(result, "|", 2)
	action := parts[0]
	url := ""
	if len(parts) > 1 {
		url = parts[1]
	}

	switch action {
	case "save":
		return url, false, nil
	case "reregister":
		return url, true, nil
	default:
		return "", false, nil
	}
}
