package cli

import (
	"fmt"
	"os"

	"sqlterm/internal/ai"
	"sqlterm/internal/config"
	"sqlterm/internal/conversation"
	"sqlterm/internal/core"
	"sqlterm/internal/i18n"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "sqlterm",
	Short: "A terminal-based SQL database tool",
	Long:  `SQLTerm provides an intuitive conversation-style interface for managing database connections and executing queries across MySQL, PostgreSQL, and SQLite.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConversation()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.sqlterm.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(addCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".sqlterm")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

func runConversation() error {
	app, err := conversation.NewApp()
	if err != nil {
		return fmt.Errorf("failed to create conversation app: %w", err)
	}
	return app.Run()
}

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to a database directly",
	RunE: func(cmd *cobra.Command, args []string) error {
		dbType, _ := cmd.Flags().GetString("db-type")
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		database, _ := cmd.Flags().GetString("database")
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")

		dbTypeEnum, err := core.ParseDatabaseType(dbType)
		if err != nil {
			return err
		}

		if port == 0 {
			port = core.GetDefaultPort(dbTypeEnum)
		}

		config := &core.ConnectionConfig{
			Name:         fmt.Sprintf("%s Connection", dbType),
			DatabaseType: dbTypeEnum,
			Host:         host,
			Port:         port,
			Database:     database,
			Username:     username,
			Password:     password,
			SSL:          false,
		}

		return connectAndRunConversation(config)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved connections",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listConnections()
	},
}

var addCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new connection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		dbType, _ := cmd.Flags().GetString("db-type")
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		database, _ := cmd.Flags().GetString("database")
		username, _ := cmd.Flags().GetString("username")

		dbTypeEnum, err := core.ParseDatabaseType(dbType)
		if err != nil {
			return err
		}

		if port == 0 {
			port = core.GetDefaultPort(dbTypeEnum)
		}

		config := &core.ConnectionConfig{
			Name:         name,
			DatabaseType: dbTypeEnum,
			Host:         host,
			Port:         port,
			Database:     database,
			Username:     username,
			SSL:          false,
		}

		return addConnection(config)
	},
}

func init() {
	connectCmd.Flags().StringP("db-type", "t", "", "Database type (mysql, postgres, sqlite)")
	connectCmd.Flags().StringP("host", "H", "localhost", "Host")
	connectCmd.Flags().IntP("port", "p", 0, "Port")
	connectCmd.Flags().StringP("database", "d", "", "Database name")
	connectCmd.Flags().StringP("username", "u", "", "Username")
	connectCmd.Flags().StringP("password", "P", "", "Password")
	connectCmd.MarkFlagRequired("db-type")
	connectCmd.MarkFlagRequired("database")
	connectCmd.MarkFlagRequired("username")

	addCmd.Flags().StringP("db-type", "t", "", "Database type (mysql, postgres, sqlite)")
	addCmd.Flags().StringP("host", "H", "localhost", "Host")
	addCmd.Flags().IntP("port", "p", 0, "Port")
	addCmd.Flags().StringP("database", "d", "", "Database name")
	addCmd.Flags().StringP("username", "u", "", "Username")
	addCmd.MarkFlagRequired("db-type")
	addCmd.MarkFlagRequired("database")
	addCmd.MarkFlagRequired("username")
}

func connectAndRunConversation(connConfig *core.ConnectionConfig) error {
	// Initialize i18n manager for CLI
	configMgr := config.NewManager()
	language := "en_au" // Default language
	
	// Try to get language from AI config
	if aiManager, err := ai.NewManager(configMgr.GetConfigDir()); err == nil && aiManager != nil {
		if aiConfig := aiManager.GetConfig(); aiConfig != nil {
			language = aiConfig.Language
		}
	}
	
	i18nMgr, err := i18n.NewManager(language)
	if err != nil {
		// Fallback to default language if i18n fails
		i18nMgr, _ = i18n.NewManager("en_au")
	}

	fmt.Printf(i18nMgr.Get("connecting_to"), connConfig.Name)

	conn, err := core.NewConnection(connConfig)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	fmt.Printf(i18nMgr.Get("connected_successfully"), connConfig.Name)
	fmt.Print(i18nMgr.Get("starting_conversation_mode"))

	app, err := conversation.NewApp()
	if err != nil {
		return fmt.Errorf("failed to create conversation app: %w", err)
	}

	app.SetConnection(conn, connConfig)
	return app.Run()
}

func listConnections() error {
	configManager := config.NewManager()
	connections, err := configManager.ListConnections()
	if err != nil {
		return fmt.Errorf("failed to load connections: %w", err)
	}

	if len(connections) == 0 {
		fmt.Println("No saved connections found.")
		fmt.Println("Add a connection with: sqlterm add <name> --db-type <type> --host <host> --database <db> --username <user>")
		return nil
	}

	fmt.Println("Saved connections:")
	for i, conn := range connections {
		fmt.Printf("%d. %s (%s) - %s://%s:%d/%s\n",
			i+1,
			conn.Name,
			conn.DatabaseType,
			conn.DatabaseType.String(),
			conn.Host,
			conn.Port,
			conn.Database)
	}

	return nil
}

func addConnection(cfg *core.ConnectionConfig) error {
	fmt.Printf("Testing connection to %s...\n", cfg.Name)

	conn, err := core.NewConnection(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	fmt.Println("✓ Connection test successful")

	configManager := config.NewManager()
	if err := configManager.SaveConnection(cfg); err != nil {
		return fmt.Errorf("failed to save connection: %w", err)
	}

	fmt.Printf("✓ Connection '%s' saved\n", cfg.Name)
	fmt.Println("Use 'sqlterm list' to see all connections")
	fmt.Println("Use 'sqlterm' to start the conversation interface")

	return nil
}
